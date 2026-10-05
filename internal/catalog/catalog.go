// SPDX-License-Identifier: Apache-2.0

// Package catalog reads, checks, and indexes the Twigs of this repository:
// one Twig per twigs/<name>/twig.yaml, validated against the JSON Schemas in
// schema/, and catalog.json, the index a Twig Nest reads.
package catalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

const (
	// SchemaBase is where schema/ is published; every schema's $id starts
	// with it, so editors and validators resolve the references there.
	SchemaBase = "https://raw.githubusercontent.com/twig-nest/twigs/main/schema/"
	// RawBase is where a Nest fetches the Twig files catalog.json names.
	RawBase = "https://raw.githubusercontent.com/twig-nest/twigs/main/"
	// Homepage is the Catalog's page.
	Homepage = "https://github.com/twig-nest/twigs"
	// IndexFile is the index a Nest reads.
	IndexFile = "catalog.json"
	// Core is the Twig every Nest installs at start.
	Core = "core"
)

// Kinds are what a Twig provides, as catalog.json names them.
var Kinds = []string{"agents", "authenticator_types", "hop_kinds", "tools"}

// Twig is one Twig file.
type Twig struct {
	// Name is the directory's name; File the file's path from the root.
	Name, File string
	// Raw is the file as it is; Doc the file as JSON.
	Raw []byte
	Doc map[string]any
}

// Problem is one thing wrong with the Catalog, at a file and, when known,
// a JSON pointer in it.
type Problem struct {
	File, Pointer, Message string
}

func (p Problem) Error() string {
	if p.Pointer != "" {
		return fmt.Sprintf("%s: %s: %s", p.File, p.Pointer, p.Message)
	}
	return fmt.Sprintf("%s: %s", p.File, p.Message)
}

// Catalog is the repository's Twigs and schemas.
type Catalog struct {
	Root  string
	Twigs []Twig
	twig  *jsonschema.Schema
	index *jsonschema.Schema
}

// Load reads the schemas under root/schema and every root/twigs/*/twig.yaml.
// A file that does not parse is a Problem; an unreadable tree is an error.
func Load(root string) (*Catalog, []Problem, error) {
	c := &Catalog{Root: root}
	var err error
	if c.twig, c.index, err = compileSchemas(filepath.Join(root, "schema")); err != nil {
		return nil, nil, err
	}
	files, err := filepath.Glob(filepath.Join(root, "twigs", "*", "twig.yaml"))
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(files)
	var problems []Problem
	for _, f := range files {
		rel, _ := filepath.Rel(root, f)
		rel = filepath.ToSlash(rel)
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		doc, err := Decode(raw)
		if err != nil {
			problems = append(problems, Problem{File: rel, Message: err.Error()})
			continue
		}
		c.Twigs = append(c.Twigs, Twig{Name: path.Base(path.Dir(rel)), File: rel, Raw: raw, Doc: doc})
	}
	// twigs/ holds one directory per Twig, named as a Twig is, each with
	// its twig.yaml; anything else is a mistake.
	entries, _ := os.ReadDir(filepath.Join(root, "twigs"))
	for _, d := range entries {
		at := "twigs/" + d.Name()
		switch {
		case !d.IsDir():
			problems = append(problems, Problem{File: at, Message: "twigs/ holds only directories: move it to twigs/<name>/twig.yaml"})
		case !twigName.MatchString(d.Name()):
			problems = append(problems, Problem{File: at, Message: "a Twig's name is lower case letters, digits, and '-', starting with a letter or digit"})
		default:
			if _, err := os.Stat(filepath.Join(root, "twigs", d.Name(), "twig.yaml")); err != nil {
				problems = append(problems, Problem{File: at, Message: "no twig.yaml in this directory"})
			}
		}
	}
	return c, problems, nil
}

// Decode reads a Twig file (YAML or JSON) as a JSON document.
func Decode(raw []byte) (map[string]any, error) {
	var v any
	if err := yaml.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("not valid YAML: %w", err)
	}
	js, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("not representable as JSON: %w", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(js, &doc); err != nil {
		return nil, fmt.Errorf("not a mapping at the top level")
	}
	return doc, nil
}

func compileSchemas(dir string) (twig, index *jsonschema.Schema, err error) {
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	if dir, err = filepath.EvalSymlinks(dir); err != nil {
		return nil, nil, fmt.Errorf("schema/: %w", err)
	}
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".json" {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		rel, _ := filepath.Rel(dir, p)
		return c.AddResource(SchemaBase+filepath.ToSlash(rel), doc)
	})
	if err != nil {
		return nil, nil, fmt.Errorf("schema/: %w", err)
	}
	if twig, err = c.Compile(SchemaBase + "twig.json"); err != nil {
		return nil, nil, fmt.Errorf("schema/twig.json: %w", err)
	}
	if index, err = c.Compile(SchemaBase + "catalog.json"); err != nil {
		return nil, nil, fmt.Errorf("schema/catalog.json: %w", err)
	}
	return twig, index, nil
}

// twigName is a Twig's name, as the schema has it.
var twigName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// hasDir reports whether twigs/<name> exists, parsed or not.
func (c *Catalog) hasDir(name string) bool {
	st, err := os.Stat(filepath.Join(c.Root, "twigs", name))
	return err == nil && st.IsDir()
}

// Get returns the Twig named name.
func (c *Catalog) Get(name string) (*Twig, bool) {
	for i := range c.Twigs {
		if c.Twigs[i].Name == name {
			return &c.Twigs[i], true
		}
	}
	return nil, false
}

// Provides is what t provides, by kind, in file order.
func (t *Twig) Provides() map[string][]string {
	out := map[string][]string{}
	for _, k := range []struct{ kind, key string }{
		{"authenticator_types", "authenticator_type"},
		{"hop_kinds", "type"},
		{"tools", "id"},
		{"agents", "id"},
	} {
		items, _ := t.Doc[k.kind].([]any)
		for _, it := range items {
			m, _ := it.(map[string]any)
			if n, _ := m[k.key].(string); n != "" {
				out[k.kind] = append(out[k.kind], n)
			}
		}
	}
	return out
}

// Requirement is one entry of a Twig's requires.
type Requirement struct{ Name, Version string }

// Requires is what t requires.
func (t *Twig) Requires() []Requirement {
	var out []Requirement
	items, _ := t.Doc["requires"].([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		n, _ := m["name"].(string)
		v, _ := m["version"].(string)
		out = append(out, Requirement{n, v})
	}
	return out
}

// Str is a top-level string field of t.
func (t *Twig) Str(field string) string {
	s, _ := t.Doc[field].(string)
	return s
}

// Older reports whether version a is lower than b (Compare).
func Older(a, b string) bool { return Compare(a, b) < 0 }

// Compare orders two Twig versions, MAJOR.MINOR.PATCH with an optional
// suffix (the schema's pattern): numbers numerically, a pre-release
// (-rc.1) before its release, as semantic versioning does, and a dotted
// suffix (.1) after it. It returns -1, 0, or 1.
func Compare(a, b string) int {
	ca, ka, sa := splitVersion(a)
	cb, kb, sb := splitVersion(b)
	for i := range 3 {
		if c := cmpInt(ca[i], cb[i]); c != 0 {
			return c
		}
	}
	// A pre-release (-) comes before the release, a dotted suffix (.) after.
	rank := map[byte]int{'-': 0, 0: 1, '.': 2}
	if c := cmpInt(rank[ka], rank[kb]); c != 0 {
		return c
	}
	return compareIdentifiers(sa, sb)
}

// splitVersion is a version's three numbers, its suffix's kind ('-', '.',
// or 0 for none), and the suffix's dot-separated identifiers.
func splitVersion(v string) ([3]int, byte, []string) {
	var core [3]int
	rest := v
	for i := range 3 {
		n := 0
		for n < len(rest) && rest[n] >= '0' && rest[n] <= '9' {
			n++
		}
		core[i], _ = strconv.Atoi(rest[:n])
		rest = rest[n:]
		if i < 2 && strings.HasPrefix(rest, ".") {
			rest = rest[1:]
		}
	}
	if rest == "" {
		return core, 0, nil
	}
	return core, rest[0], strings.Split(rest[1:], ".")
}

// compareIdentifiers compares suffix identifiers as semantic versioning
// does pre-release ones: numeric ones numerically and before others, the
// rest by their text, and a shorter list first when one is the other's
// start.
func compareIdentifiers(a, b []string) int {
	for i := range min(len(a), len(b)) {
		na, ea := strconv.Atoi(a[i])
		nb, eb := strconv.Atoi(b[i])
		switch {
		case ea == nil && eb == nil:
			if c := cmpInt(na, nb); c != 0 {
				return c
			}
		case ea == nil:
			return -1
		case eb == nil:
			return 1
		default:
			if c := strings.Compare(a[i], b[i]); c != 0 {
				return c
			}
		}
	}
	return cmpInt(len(a), len(b))
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// entry is one Twig of catalog.json.
type entry struct {
	Name      string              `json:"name"`
	Version   string              `json:"version"`
	Summary   string              `json:"summary"`
	Publisher string              `json:"publisher"`
	Tags      []string            `json:"tags,omitempty"`
	Provides  map[string][]string `json:"provides"`
	Requires  []string            `json:"requires,omitempty"`
	URL       string              `json:"url"`
	SHA256    string              `json:"sha256"`
}

// Index renders catalog.json: every Twig with its file's URL and SHA-256.
func (c *Catalog) Index() ([]byte, error) {
	index := struct {
		Catalog  string  `json:"catalog"`
		Name     string  `json:"name"`
		Homepage string  `json:"homepage"`
		Twigs    []entry `json:"twigs"`
	}{Catalog: "twig-nest/catalog/v1", Name: "Twigs", Homepage: Homepage, Twigs: []entry{}}
	for i := range c.Twigs {
		t := &c.Twigs[i]
		sum := sha256.Sum256(t.Raw)
		e := entry{Name: t.Name, Version: t.Str("version"), Summary: t.Str("summary"), Publisher: t.Str("publisher"),
			Provides: t.Provides(), URL: RawBase + t.File, SHA256: hex.EncodeToString(sum[:])}
		tags, _ := t.Doc["tags"].([]any)
		for _, tag := range tags {
			if s, ok := tag.(string); ok {
				e.Tags = append(e.Tags, s)
			}
		}
		for _, r := range t.Requires() {
			e.Requires = append(e.Requires, r.Name)
		}
		index.Twigs = append(index.Twigs, e)
	}
	b, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return nil, err
	}
	b = append(b, '\n')
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	if err := c.index.Validate(doc); err != nil {
		return nil, fmt.Errorf("the index is not valid: %w", err)
	}
	return b, nil
}
