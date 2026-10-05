// SPDX-License-Identifier: Apache-2.0

// Command twigs checks and indexes this Catalog of Twigs.
//
//	twigs check [-base REF]   check every Twig; with -base, also that a changed
//	                          Twig raised its version since REF (a git ref)
//	twigs index [-check]      write catalog.json and the README's table; with
//	                          -check, fail when they are not current instead
//	twigs new NAME            start twigs/NAME/twig.yaml from a template
//
// Run it from the repository root (make check, make index, make new).
// Inside GitHub Actions, problems are also printed as annotations on the
// files of a pull request.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shebang-labs/twigs/internal/catalog"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, errReported) {
			fmt.Fprintln(os.Stderr, "twigs:", err)
		}
		os.Exit(1)
	}
}

// errReported is a failure whose problems were already printed.
var errReported = errors.New("reported")

const usage = `usage:
  twigs check [-base REF]   check every Twig (and version bumps since REF)
  twigs index [-check]      write catalog.json and the README's table
  twigs new NAME            start twigs/NAME/twig.yaml from a template
`

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return errReported
	}
	switch args[0] {
	case "check":
		return check(args[1:], stdout, stderr)
	case "index":
		return index(args[1:], stdout, stderr)
	case "new":
		return newTwig(args[1:], stdout)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return nil
	}
	fmt.Fprint(stderr, usage)
	return errReported
}

func report(w io.Writer, ps []catalog.Problem) error {
	if len(ps) == 0 {
		return nil
	}
	annotate := os.Getenv("GITHUB_ACTIONS") == "true"
	for _, p := range ps {
		fmt.Fprintln(w, p.Error())
		if annotate {
			msg := p.Message
			if p.Pointer != "" {
				msg = p.Pointer + ": " + msg
			}
			fmt.Fprintf(w, "::error file=%s,title=twigs check::%s\n", p.File, strings.NewReplacer("%", "%25", "\n", "%0A", "\r", "%0D").Replace(msg))
		}
	}
	fmt.Fprintf(w, "%d problem(s); CONTRIBUTING.md explains each check\n", len(ps))
	return errReported
}

func check(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	base := fs.String("base", "", "a git ref to compare versions with (a pull request's base branch)")
	if err := fs.Parse(args); err != nil {
		return errReported
	}
	c, ps, err := catalog.Load(".")
	if err != nil {
		return err
	}
	ps = append(ps, c.Check()...)
	if _, err := c.Index(); err != nil {
		ps = append(ps, catalog.Problem{File: catalog.IndexFile, Message: err.Error()})
	}
	if *base != "" {
		// A ref that does not resolve would make every Twig look new.
		if err := exec.Command("git", "rev-parse", "--verify", "--quiet", *base+"^{commit}").Run(); err != nil {
			return fmt.Errorf("-base %s is not a commit here: fetch it first (git fetch origin main, or upstream for a fork)", *base)
		}
		files, err := filesAt(*base, c)
		if err != nil {
			return err
		}
		ps = append(ps, c.CheckVersions(files)...)
		baseIndex, _ := exec.Command("git", "show", *base+":"+catalog.IndexFile).Output()
		have, err := os.ReadFile(catalog.IndexFile)
		if err != nil {
			return err
		}
		ps = append(ps, c.CheckIndex(baseIndex, have)...)
	}
	if err := report(stderr, ps); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%d Twigs checked: all pass\n", len(c.Twigs))
	return nil
}

// filesAt reads each Twig's file at a git ref; a file the ref lacks is new.
func filesAt(ref string, c *catalog.Catalog) (map[string][]byte, error) {
	out := map[string][]byte{}
	for i := range c.Twigs {
		f := c.Twigs[i].File
		b, err := exec.Command("git", "show", ref+":"+f).Output()
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				continue
			}
			return nil, fmt.Errorf("git show %s:%s: %w", ref, f, err)
		}
		out[f] = b
	}
	return out, nil
}

func index(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	only := fs.Bool("check", false, "fail when catalog.json or the README's table is not current, writing nothing")
	if err := fs.Parse(args); err != nil {
		return errReported
	}
	c, ps, err := catalog.Load(".")
	if err != nil {
		return err
	}
	if err := report(stderr, ps); err != nil {
		return err
	}
	idx, err := c.Index()
	if err != nil {
		return err
	}
	readme, err := os.ReadFile("README.md")
	if err != nil {
		return err
	}
	table, err := c.WithTable(readme)
	if err != nil {
		return err
	}
	stale := false
	for _, f := range []struct {
		name string
		want []byte
	}{{catalog.IndexFile, idx}, {"README.md", table}} {
		have, _ := os.ReadFile(f.name)
		if bytes.Equal(have, f.want) {
			continue
		}
		if *only {
			fmt.Fprintf(stderr, "%s is not current: run make index\n", f.name)
			stale = true
			continue
		}
		if err := os.WriteFile(f.name, f.want, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "wrote %s\n", f.name)
	}
	if stale {
		return errReported
	}
	return nil
}

var twigName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

func newTwig(args []string, stdout io.Writer) error {
	if len(args) != 1 || !twigName.MatchString(args[0]) {
		return errors.New("new takes one name: lower case letters, digits, and '-', such as mysql or my-tool")
	}
	dir := filepath.Join("twigs", args[0])
	file := filepath.Join(dir, "twig.yaml")
	if _, err := os.Stat(file); err == nil {
		return fmt.Errorf("%s exists", file)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(file, []byte(strings.ReplaceAll(template, "NAME", args[0])), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s: fill it in (docs/twig-format.md), then run make check\n", file)
	return nil
}

const template = `# yaml-language-server: $schema=https://raw.githubusercontent.com/shebang-labs/twigs/main/schema/twig.json
twig: twig-nest/twig/v1
name: NAME
version: 1.0.0
summary: One line on what NAME adds to a Nest
publisher: Your name or organization
homepage: https://example.com/the-project-it-wraps
license: Apache-2.0
tags: [example]
# Keep what it provides and delete the rest (docs/twig-format.md).
tools:
- id: NAME
  version: 1.0.0
  summary: The NAME CLI
  install: |
    ARG TARGETARCH
    RUN set -eu; \
        case "${TARGETARCH}" in \
          amd64) url=https://example.com/NAME-1.0.0-linux-amd64 sha=<sha256 of that file> ;; \
          arm64) url=https://example.com/NAME-1.0.0-linux-arm64 sha=<sha256 of that file> ;; \
          *) echo "NAME: no build for ${TARGETARCH}" >&2; exit 1 ;; \
        esac; \
        curl --proto '=https' --tlsv1.2 -fsSL --retry 3 -o /tmp/NAME.dl "${url}"; \
        echo "${sha}  /tmp/NAME.dl" | sha256sum --check --strict -; \
        install -m 0755 /tmp/NAME.dl /usr/local/bin/NAME; \
        rm -f /tmp/NAME.dl
  provides:
  - binary: NAME
    version_command: NAME --version
  notes: 'The license of what it installs. Checksums: where they come from.'
`
