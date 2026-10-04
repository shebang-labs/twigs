// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Check runs every check a Twig must pass before a Hub is offered it, the
// ones a Hub runs on install among them, and returns what fails.
func (c *Catalog) Check() []Problem {
	var ps []Problem
	// Where each name is provided: a name belongs to one place.
	type at struct{ file, ptr, twig string }
	owners := map[string][]at{}
	for i := range c.Twigs {
		t := &c.Twigs[i]
		ps = append(ps, c.checkSchema(t)...)
		if n := t.Str("name"); n != t.Name {
			ps = append(ps, Problem{t.File, "/name", fmt.Sprintf("is %q; it must be the directory's name, %q", n, t.Name)})
		}
		for _, kind := range Kinds {
			for j, n := range t.Provides()[kind] {
				key := kind + "/" + n
				owners[key] = append(owners[key], at{t.File, fmt.Sprintf("/%s/%d", kind, j), t.Name})
			}
		}
		ps = append(ps, toolRules(t)...)
		ps = append(ps, agentRules(t)...)
		ps = append(ps, placeholderRules(t)...)
	}
	keys := make([]string, 0, len(owners))
	for k := range owners {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		places := owners[key]
		if len(places) < 2 {
			continue
		}
		var where []string
		for _, o := range places {
			where = append(where, "Twig "+o.twig+" ("+o.ptr+")")
		}
		_, name, _ := strings.Cut(key, "/")
		for _, o := range places {
			ps = append(ps, Problem{o.file, o.ptr, fmt.Sprintf("%s is provided %d times: %s; a name belongs to one place", name, len(places), strings.Join(where, ", "))})
		}
	}
	ps = append(ps, c.checkRequires()...)
	ps = append(ps, c.checkAgentTools()...)
	if _, ok := c.Get(Core); !ok && !c.hasDir(Core) {
		ps = append(ps, Problem{File: "twigs/" + Core, Message: "there is no core Twig, which every Hub installs at start"})
	}
	return ps
}

func (c *Catalog) checkSchema(t *Twig) []Problem {
	if v, ok := t.Doc["version"]; ok {
		if _, isString := v.(string); !isString {
			return []Problem{{t.File, "/version", fmt.Sprintf("is %v, which YAML reads as a number: quote it, as in version: \"1.0.0\"", v)}}
		}
	}
	js, _ := json.Marshal(t.Doc)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(js))
	if err != nil {
		return []Problem{{File: t.File, Message: err.Error()}}
	}
	err = c.twig.Validate(doc)
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		if err != nil {
			return []Problem{{File: t.File, Message: err.Error()}}
		}
		return nil
	}
	var ps []Problem
	for _, leaf := range leaves(ve) {
		ps = append(ps, Problem{t.File, "/" + strings.Join(leaf.InstanceLocation, "/"), leaf.ErrorKind.LocalizedString(printer)})
	}
	return ps
}

// printer renders schema errors in English.
var printer = message.NewPrinter(language.English)

// leaves are the innermost causes of a validation error: the ones that say
// what to fix.
func leaves(e *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(e.Causes) == 0 {
		return []*jsonschema.ValidationError{e}
	}
	var out []*jsonschema.ValidationError
	for _, c := range e.Causes {
		out = append(out, leaves(c)...)
	}
	return out
}

// checkRequires: every requirement is a Twig of this Catalog at the version
// asked for or newer, and no Twig requires itself through others.
func (c *Catalog) checkRequires() []Problem {
	var ps []Problem
	for i := range c.Twigs {
		t := &c.Twigs[i]
		for j, r := range t.Requires() {
			ptr := fmt.Sprintf("/requires/%d", j)
			req, ok := c.Get(r.Name)
			switch {
			case !ok:
				ps = append(ps, Problem{t.File, ptr, fmt.Sprintf("requires %s, which this Catalog does not have", r.Name)})
			case r.Version != "" && Older(req.Str("version"), r.Version):
				ps = append(ps, Problem{t.File, ptr, fmt.Sprintf("requires %s %s or newer; this Catalog has %s", r.Name, r.Version, req.Str("version"))})
			}
		}
		if loop := c.loopFrom(t.Name, []string{t.Name}); loop != nil {
			ps = append(ps, Problem{t.File, "/requires", "a loop: " + strings.Join(loop, " requires ")})
		}
	}
	return ps
}

func (c *Catalog) loopFrom(start string, path []string) []string {
	t, ok := c.Get(path[len(path)-1])
	if !ok {
		return nil
	}
	for _, r := range t.Requires() {
		if r.Name == start {
			return append(slices.Clone(path), start)
		}
		if slices.Contains(path, r.Name) {
			continue
		}
		if loop := c.loopFrom(start, append(slices.Clone(path), r.Name)); loop != nil {
			return loop
		}
	}
	return nil
}

// closure is name and every Twig it requires, directly or not.
func (c *Catalog) closure(name string, seen map[string]bool) map[string]bool {
	if seen == nil {
		seen = map[string]bool{}
	}
	if seen[name] {
		return seen
	}
	seen[name] = true
	if t, ok := c.Get(name); ok {
		for _, r := range t.Requires() {
			c.closure(r.Name, seen)
		}
	}
	return seen
}

// checkAgentTools: an Agent's Tool comes with its Twig or one it requires,
// and puts the Agent's binary on PATH.
func (c *Catalog) checkAgentTools() []Problem {
	var ps []Problem
	for i := range c.Twigs {
		t := &c.Twigs[i]
		agents, _ := t.Doc["agents"].([]any)
		for j, a := range agents {
			am, _ := a.(map[string]any)
			tool, _ := am["tool"].(string)
			binary, _ := am["binary"].(string)
			if tool == "" {
				continue
			}
			ptr := fmt.Sprintf("/agents/%d/tool", j)
			found := false
			for n := range c.closure(t.Name, nil) {
				o, _ := c.Get(n)
				if o == nil {
					continue
				}
				tools, _ := o.Doc["tools"].([]any)
				for _, tl := range tools {
					tm, _ := tl.(map[string]any)
					if id, _ := tm["id"].(string); id != tool {
						continue
					}
					found = true
					provides, _ := tm["provides"].([]any)
					has := false
					for _, p := range provides {
						pm, _ := p.(map[string]any)
						has = has || pm["binary"] == binary
					}
					if !has {
						ps = append(ps, Problem{t.File, ptr, fmt.Sprintf("Tool %s does not provide %s, the Agent's binary", tool, binary)})
					}
				}
			}
			if !found {
				ps = append(ps, Problem{t.File, ptr, fmt.Sprintf("Tool %s is neither in this Twig nor in one it requires", tool)})
			}
		}
	}
	return ps
}

// toolInstructions are the Dockerfile instructions a Tool's fragment may
// hold; the Hub's renderer owns FROM, USER, and ENTRYPOINT.
var toolInstructions = map[string]bool{"RUN": true, "COPY": true, "ENV": true, "ARG": true, "WORKDIR": true}

// pinnedCopyFrom is COPY --from=<image>@sha256:<digest>.
var pinnedCopyFrom = regexp.MustCompile(`^--from=[a-z0-9][a-z0-9._/:-]*@sha256:[a-f0-9]{64}$`)

// toolRules are the Hub's rules for a Tool's install fragment: only the
// allowed instructions, a COPY only from an image pinned by digest, no
// build argument that looks like a secret, and one entry per binary.
func toolRules(t *Twig) []Problem {
	var ps []Problem
	tools, _ := t.Doc["tools"].([]any)
	for j, tl := range tools {
		tm, _ := tl.(map[string]any)
		at := fmt.Sprintf("/tools/%d", j)
		install, _ := tm["install"].(string)
		if strings.Contains(install, "<sha256") || strings.Contains(install, "https://example.com/") {
			ps = append(ps, Problem{t.File, at + "/install", "still has the template's placeholders: the real download URLs and their SHA-256 checksums"})
		}
		for k, line := range fragmentInstructions(install) {
			word, rest, _ := strings.Cut(line, " ")
			instr := strings.ToUpper(word)
			switch {
			case !toolInstructions[instr]:
				ps = append(ps, Problem{t.File, at + "/install", fmt.Sprintf("instruction %d is %s; a fragment holds RUN, COPY --from=<image>@sha256:<digest>, ENV, ARG, and WORKDIR only (the Hub owns FROM, USER, and ENTRYPOINT)", k+1, word)})
			case instr == "COPY":
				from, _, _ := strings.Cut(strings.TrimSpace(rest), " ")
				if !pinnedCopyFrom.MatchString(from) {
					ps = append(ps, Problem{t.File, at + "/install", fmt.Sprintf("instruction %d copies from %q; a fragment copies only from an image pinned by digest", k+1, from)})
				}
			case instr == "ARG" || instr == "ENV":
				n := strings.ToUpper(strings.TrimSpace(rest))
				if strings.Contains(n, "TOKEN") || strings.Contains(n, "PASSWORD") || strings.Contains(n, "SECRET") || strings.Contains(n, "_KEY") {
					ps = append(ps, Problem{t.File, at + "/install", fmt.Sprintf("instruction %d (%s) looks like it carries a secret; a fragment carries none", k+1, instr)})
				}
			}
		}
		seen := map[string]bool{}
		provides, _ := tm["provides"].([]any)
		for k, p := range provides {
			pm, _ := p.(map[string]any)
			bin, _ := pm["binary"].(string)
			if seen[bin] {
				ps = append(ps, Problem{t.File, fmt.Sprintf("%s/provides/%d/binary", at, k), bin + " is listed twice"})
			}
			seen[bin] = true
		}
	}
	return ps
}

// fragmentInstructions splits a Dockerfile fragment into its
// instructions: comments and blank lines dropped, continuations joined.
func fragmentInstructions(fragment string) []string {
	var out []string
	var cur strings.Builder
	for _, line := range strings.Split(fragment, "\n") {
		t := strings.TrimSpace(line)
		// Docker drops comment lines and empty lines, also inside an
		// instruction continued over several lines.
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasSuffix(t, "\\") {
			cur.WriteString(strings.TrimSuffix(t, "\\"))
			cur.WriteString(" ")
			continue
		}
		cur.WriteString(t)
		out = append(out, cur.String())
		cur.Reset()
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// agentPlaceholder is a {name} in an Agent's arguments; the Runner sets
// only agentPlaceholders.
var (
	agentPlaceholder  = regexp.MustCompile(`\{([a-z_.]+)\}`)
	agentPlaceholders = map[string]bool{"model": true, "budget_usd": true, "system_prompt": true, "job.dir": true}
)

func agentRules(t *Twig) []Problem {
	var ps []Problem
	agents, _ := t.Doc["agents"].([]any)
	for j, a := range agents {
		am, _ := a.(map[string]any)
		args, _ := am["args"].([]any)
		for k, arg := range args {
			s, _ := arg.(string)
			for _, m := range agentPlaceholder.FindAllStringSubmatch(s, -1) {
				if !agentPlaceholders[m[1]] {
					ps = append(ps, Problem{t.File, "/agents/" + strconv.Itoa(j) + "/args/" + strconv.Itoa(k),
						"{" + m[1] + "} is not a placeholder the Runner sets ({model}, {budget_usd}, {system_prompt}, {job.dir})"})
				}
			}
		}
	}
	return ps
}

// CheckVersions compares each Twig with its file at a base (the base
// branch of a pull request): a changed file must raise its version, since
// a Hub installs a file by its digest and offers a newer version as an
// upgrade. base maps a file to its content at the base, absent when new.
func (c *Catalog) CheckVersions(base map[string][]byte) []Problem {
	var ps []Problem
	for i := range c.Twigs {
		t := &c.Twigs[i]
		old, ok := base[t.File]
		if !ok || bytes.Equal(old, t.Raw) {
			continue
		}
		was, err := Decode(old)
		if err != nil {
			continue
		}
		prev, _ := was["version"].(string)
		switch cur := t.Str("version"); Compare(prev, cur) {
		case 0:
			ps = append(ps, Problem{t.File, "/version", fmt.Sprintf("is %s, as before the change; a changed Twig raises its version, so Hubs offer it as an upgrade", cur)})
		case 1:
			ps = append(ps, Problem{t.File, "/version", fmt.Sprintf("went down from %s to %s; a changed Twig raises its version", prev, cur)})
		}
	}
	return ps
}

// CheckIndex refuses a hand-made catalog.json: the index workflow writes it
// after a merge, so a pull request leaves it as the base has it, or makes
// it what make index writes.
func (c *Catalog) CheckIndex(base, have []byte) []Problem {
	if bytes.Equal(base, have) {
		return nil
	}
	want, err := c.Index()
	if err != nil || !bytes.Equal(want, have) {
		return []Problem{{File: IndexFile, Message: "was edited by hand: leave it as it is (a workflow rewrites it after the merge), or run make index"}}
	}
	return nil
}

// placeholder is a {name} or {scope.name} in a template.
var placeholder = regexp.MustCompile(`\{([a-z_]+)(?:\.([a-zA-Z0-9_-]+))?\}`)

// placeholderRules: a Credential type's or Hop kind's {params.x},
// {secrets.x}, and {file.x} name a param, secret slot, or file it declares
// (or a variant does), and an Agent's templates use only what the Runner
// sets. A misspelled one would reach the Runner unexpanded.
func placeholderRules(t *Twig) []Problem {
	var ps []Problem
	for _, kind := range []string{"authenticator_types", "hop_kinds"} {
		items, _ := t.Doc[kind].([]any)
		for j, it := range items {
			m, _ := it.(map[string]any)
			declared := map[string]map[string]bool{"params": {}, "secrets": {}, "file": {}}
			collectDeclared(m, declared)
			if vs, ok := m["variants"].(map[string]any); ok {
				if vals, ok := vs["values"].(map[string]any); ok {
					for _, v := range vals {
						if vm, ok := v.(map[string]any); ok {
							collectDeclared(vm, declared)
						}
					}
				}
			}
			walkStrings(m, fmt.Sprintf("/%s/%d", kind, j), func(ptr, str string) {
				for _, p := range placeholder.FindAllStringSubmatch(str, -1) {
					names, scoped := declared[p[1]]
					if scoped && p[2] != "" && !names[p[2]] {
						ps = append(ps, Problem{t.File, ptr, fmt.Sprintf("{%s.%s} names no %s this declares", p[1], p[2], map[string]string{"params": "param", "secrets": "secret slot", "file": "file"}[p[1]])})
					}
				}
			})
		}
	}
	agents, _ := t.Doc["agents"].([]any)
	for j, a := range agents {
		am, _ := a.(map[string]any)
		for _, field := range []string{"env", "acp"} {
			walkStrings(am[field], fmt.Sprintf("/agents/%d/%s", j, field), func(ptr, str string) {
				for _, p := range agentPlaceholder.FindAllStringSubmatch(str, -1) {
					if !agentPlaceholders[p[1]] {
						ps = append(ps, Problem{t.File, ptr, "{" + p[1] + "} is not a placeholder the Runner sets ({model}, {budget_usd}, {system_prompt}, {job.dir})"})
					}
				}
			})
		}
	}
	return ps
}

func collectDeclared(m map[string]any, declared map[string]map[string]bool) {
	if p, ok := m["params"].(map[string]any); ok {
		if props, ok := p["properties"].(map[string]any); ok {
			for k := range props {
				declared["params"][k] = true
			}
		}
	}
	if sec, ok := m["secrets"].(map[string]any); ok {
		for k := range sec {
			declared["secrets"][k] = true
		}
	}
	if files, ok := m["files"].([]any); ok {
		for _, f := range files {
			if fm, ok := f.(map[string]any); ok {
				if n, ok := fm["name"].(string); ok {
					declared["file"][n] = true
				}
			}
		}
	}
}

// walkStrings calls f with every string under v and its JSON pointer,
// leaving out JSON Schemas (params), whose text is not a template.
func walkStrings(v any, ptr string, f func(ptr, s string)) {
	switch x := v.(type) {
	case string:
		f(ptr, x)
	case map[string]any:
		for k, e := range x {
			if k == "params" {
				continue
			}
			walkStrings(e, ptr+"/"+k, f)
		}
	case []any:
		for i, e := range x {
			walkStrings(e, ptr+"/"+strconv.Itoa(i), f)
		}
	}
}
