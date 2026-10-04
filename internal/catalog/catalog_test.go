// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepository: this repository's Twigs pass every check and index.
// catalog.json and the README's table may lag a pull request: the index
// workflow rewrites them on main.
func TestRepository(t *testing.T) {
	c, ps, err := Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	ps = append(ps, c.Check()...)
	for _, p := range ps {
		t.Error(p)
	}
	if _, err := c.Index(); err != nil {
		t.Fatal(err)
	}
	readme, _ := os.ReadFile("../../README.md")
	if _, err := c.WithTable(readme); err != nil {
		t.Fatal(err)
	}
}

// fixture writes a catalog of the given Twig files beside this
// repository's schemas and loads it.
func fixture(t *testing.T, twigs map[string]string) (*Catalog, []Problem) {
	t.Helper()
	root := t.TempDir()
	if err := os.Symlink(mustAbs(t, "../../schema"), filepath.Join(root, "schema")); err != nil {
		t.Fatal(err)
	}
	for name, body := range twigs {
		dir := filepath.Join(root, "twigs", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "twig.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, ps, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return c, append(ps, c.Check()...)
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	a, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func head(name, version string) string {
	return "twig: twig-nest/twig/v1\nname: " + name + "\nversion: " + version + "\nsummary: x\npublisher: test\n"
}

const core = "twig: twig-nest/twig/v1\nname: core\nversion: 1.0.0\nsummary: x\npublisher: test\n"

func tool(id, install, binary string) string {
	return "tools:\n- id: " + id + "\n  version: 1.0.0\n  summary: x\n  install: |\n    " + strings.ReplaceAll(install, "\n", "\n    ") + "\n  provides:\n  - binary: " + binary + "\n"
}

// wants checks that ps holds exactly one problem per substring.
func wants(t *testing.T, ps []Problem, subs ...string) {
	t.Helper()
	if len(ps) != len(subs) {
		t.Errorf("%d problems, want %d: %v", len(ps), len(subs), ps)
	}
	for _, s := range subs {
		found := false
		for _, p := range ps {
			found = found || strings.Contains(p.Error(), s)
		}
		if !found {
			t.Errorf("no problem says %q: %v", s, ps)
		}
	}
}

func TestChecks(t *testing.T) {
	for _, c := range []struct {
		name  string
		twigs map[string]string
		want  []string
	}{
		{"valid", map[string]string{"core": core, "a": head("a", "1.0.0") + tool("a", "RUN true", "a")}, nil},
		{"no core", map[string]string{"a": head("a", "1.0.0")}, []string{"no core Twig"}},
		{"not YAML", map[string]string{"core": core, "a": "name: [a"}, []string{"twigs/a/twig.yaml: not valid YAML"}},
		{"schema", map[string]string{"core": core, "a": head("a", "one")}, []string{"twigs/a/twig.yaml: /version"}},
		{"unknown field", map[string]string{"core": core, "a": head("a", "1.0.0") + "colour: red\n"}, []string{"colour"}},
		{"name is the directory's", map[string]string{"core": core, "a": head("b", "1.0.0")}, []string{"/name: is \"b\""}},
		{"a name is one Twig's", map[string]string{"core": core, "a": head("a", "1.0.0") + tool("x", "RUN true", "x"), "b": head("b", "1.0.0") + tool("x", "RUN true", "x")},
			[]string{"twigs/a/twig.yaml: /tools/0: x is provided 2 times", "twigs/b/twig.yaml: /tools/0: x is provided 2 times"}},
		{"a name twice in one Twig", map[string]string{"core": core, "a": head("a", "1.0.0") + tool("x", "RUN true", "x") + "- id: x\n  version: 1.0.0\n  install: RUN true\n  provides:\n  - binary: y\n"},
			[]string{"/tools/0: x is provided 2 times", "/tools/1: x is provided 2 times"}},
		{"a version YAML reads as a number", map[string]string{"core": core, "a": "twig: twig-nest/twig/v1\nname: a\nversion: 1.0\nsummary: x\npublisher: x\n"}, []string{"quote it"}},
		{"a comment inside a continued RUN", map[string]string{"core": core, "a": head("a", "1.0.0") + tool("a", "RUN set -eu; \\\n    # a note\n    true", "a")}, nil},
		{"a misspelled param", map[string]string{"core": core, "a": head("a", "1.0.0") + "authenticator_types:\n- authenticator_type: a.b\n  version: 1\n  provider: a\n  summary: x\n  interactive: false\n  permissions_hint: x\n  params: {type: object, properties: {host: {type: string}}}\n  usage: a -h {params.hots}\n"},
			[]string{"{params.hots} names no param"}},
		{"an Agent env placeholder", map[string]string{"core": core, "t": head("t", "1.0.0") + tool("a", "RUN true", "a"), "a": head("a", "1.0.0") + "requires: [{name: t}]\nagents:\n- id: a\n  binary: a\n  args: []\n  tool: a\n  env: {HOME_DIR: \"{jobdir}\"}\n"},
			[]string{"{jobdir} is not a placeholder"}},
		{"requires a missing Twig", map[string]string{"core": core, "a": head("a", "1.0.0") + "requires: [{name: ghost}]\n"}, []string{"requires ghost"}},
		{"requires a newer version", map[string]string{"core": core, "a": head("a", "1.0.0") + "requires: [{name: core, version: 2.0.0}]\n"}, []string{"core 2.0.0 or newer"}},
		{"a loop", map[string]string{"core": core, "a": head("a", "1.0.0") + "requires: [{name: b}]\n", "b": head("b", "1.0.0") + "requires: [{name: a}]\n"},
			[]string{"twigs/a/twig.yaml: /requires: a loop: a requires b requires a", "twigs/b/twig.yaml: /requires: a loop: b requires a requires b"}},
		{"FROM in a fragment", map[string]string{"core": core, "a": head("a", "1.0.0") + tool("a", "FROM debian", "a")}, []string{"instruction 1 is FROM"}},
		{"COPY from the context", map[string]string{"core": core, "a": head("a", "1.0.0") + tool("a", "COPY a /a", "a")}, []string{"copies from \"a\""}},
		{"template placeholders", map[string]string{"core": core, "a": head("a", "1.0.0") + tool("a", "RUN curl https://example.com/a # sha=<sha256 of that file>", "a")}, []string{"template's placeholders"}},
		{"a secret ARG", map[string]string{"core": core, "a": head("a", "1.0.0") + tool("a", "ARG GITHUB_TOKEN\nRUN true", "a")}, []string{"looks like it carries a secret"}},
		{"an Agent's Tool is missing", map[string]string{"core": core, "a": head("a", "1.0.0") + "agents:\n- id: a\n  binary: a\n  args: []\n  tool: ghost\n  summary: x\n  version_command: a --version\n"},
			[]string{"Tool ghost is neither"}},
		{"an Agent's Tool lacks its binary", map[string]string{"core": core, "a": head("a", "1.0.0") + tool("a", "RUN true", "other") + "agents:\n- id: a\n  binary: a\n  args: []\n  tool: a\n  summary: x\n  version_command: a --version\n"},
			[]string{"does not provide a"}},
		{"an Agent's Tool from a required Twig", map[string]string{"core": core, "t": head("t", "1.0.0") + tool("a", "RUN true", "a"), "a": head("a", "1.0.0") + "requires: [{name: t}]\nagents:\n- id: a\n  binary: a\n  args: []\n  tool: a\n  summary: x\n  version_command: a --version\n"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, ps := fixture(t, c.twigs)
			wants(t, ps, c.want...)
		})
	}
}

// TestCheckVersions: any change to a Twig's file, a comment included (it
// changes the file's digest), raises its version; an unchanged or new
// file needs none.
func TestCheckVersions(t *testing.T) {
	a, b := head("a", "1.0.0")+"tags: [x]\n", head("b", "1.1.0")+"tags: [y]\n"
	c, ps := fixture(t, map[string]string{"core": core, "a": a, "b": b})
	wants(t, ps)
	base := map[string][]byte{
		"twigs/a/twig.yaml": []byte(a),                                   // unchanged
		"twigs/b/twig.yaml": []byte(head("b", "1.0.0") + "tags: [z]\n"), // changed and raised
	}
	wants(t, c.CheckVersions(base))
	base["twigs/a/twig.yaml"] = []byte("# a comment\n" + a)               // a comment only
	base["twigs/b/twig.yaml"] = []byte(head("b", "1.2.0") + "tags: [z]\n") // lowered
	wants(t, c.CheckVersions(base), "twigs/a/twig.yaml: /version: is 1.0.0, as before", "twigs/b/twig.yaml: /version: went down from 1.2.0 to 1.1.0")
}

// TestCheckIndex: a pull request leaves catalog.json as the base has it,
// or makes it the generated index; any other edit is refused.
func TestCheckIndex(t *testing.T) {
	c, _ := fixture(t, map[string]string{"core": core})
	idx, err := c.Index()
	if err != nil {
		t.Fatal(err)
	}
	wants(t, c.CheckIndex([]byte("old"), []byte("old")))
	wants(t, c.CheckIndex([]byte("old"), idx))
	wants(t, c.CheckIndex([]byte("old"), []byte("tampered")), "catalog.json: was edited by hand")
}

// TestLayout: twigs/ holds only Twig directories with a twig.yaml.
func TestLayout(t *testing.T) {
	_, ps := fixture(t, map[string]string{"core": core, "Bad": head("Bad", "1.0.0")})
	found := false
	for _, p := range ps {
		found = found || strings.Contains(p.Error(), "twigs/Bad: a Twig's name is lower case")
	}
	if !found {
		t.Errorf("no layout problem: %v", ps)
	}
}

func TestCompare(t *testing.T) {
	// Each version is lower than the next.
	ordered := []string{"0.9.0", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.0.1", "1.0.1", "1.9.0", "1.10.0", "2.0.0"}
	for i := range ordered {
		for j := range ordered {
			want := cmpInt(i, j)
			if got := Compare(ordered[i], ordered[j]); got != want {
				t.Errorf("Compare(%s, %s) = %d, want %d", ordered[i], ordered[j], got, want)
			}
		}
	}
}
