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
			[]string{"twigs/b/twig.yaml: /tools/0: x is already Twig a's"}},
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

// TestCheckVersions: a Twig whose content changed raises its version; a
// comment or formatting change, or a new Twig, needs none.
func TestCheckVersions(t *testing.T) {
	c, ps := fixture(t, map[string]string{"core": core, "a": "# a comment\n" + head("a", "1.0.0") + "tags: [x]\n", "b": head("b", "1.1.0") + "tags: [y]\n"})
	wants(t, ps)
	base := map[string][]byte{
		"twigs/a/twig.yaml": []byte(head("a", "1.0.0") + "tags:\n- x\n"), // reformatted only
		"twigs/b/twig.yaml": []byte(head("b", "1.0.0") + "tags: [z]\n"),  // changed and raised
	}
	wants(t, c.CheckVersions(base))
	base["twigs/b/twig.yaml"] = []byte(head("b", "1.1.0") + "tags: [z]\n") // changed, not raised
	wants(t, c.CheckVersions(base), "twigs/b/twig.yaml: /version: is 1.1.0, as before")
}

func TestOlder(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"1.0.0", "1.0.1", true}, {"1.9.0", "1.10.0", true}, {"2.0.0", "1.9.9", false}, {"1.0.0", "1.0.0", false}, {"1.0", "1.0.1", true}} {
		if got := Older(c.a, c.b); got != c.want {
			t.Errorf("Older(%s, %s) = %v", c.a, c.b, got)
		}
	}
}
