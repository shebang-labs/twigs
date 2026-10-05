# Security

## Reporting a vulnerability

Report it privately through [GitHub's private vulnerability reporting](https://github.com/twig-nest/twigs/security/advisories/new) (the Security tab, "Report a vulnerability"). Do not open a public issue or pull request for it.

Say which Twig and version, what an attacker could do, and how to reproduce it. A maintainer answers within three working days, keeps you informed, and credits you in the advisory unless you ask otherwise.

In scope: a Twig that installs something other than what it says, a download that is not pinned or checked, a Credential type whose checks write or leak a secret, an instruction that weakens a Runner, and anything in the checker or the workflows that lets a pull request change what a Nest installs without review.

## How the catalog protects Nests

- Every download is pinned to a version and checked against its SHA-256 before use.
- A Nest installs a Twig only when the file's SHA-256 still matches `catalog.json`, and shows an admin everything the Twig provides and every command it runs before installing it.
- `main` takes changes only through pull requests that pass CI and a code owner approved ([CODEOWNERS](.github/CODEOWNERS), [docs/reviewing.md](docs/reviewing.md)); workflow tokens are read-only, and only the index workflow, after a merge, writes to `main`, with a deploy key kept for that alone.
- Actions are pinned by commit, and Dependabot keeps them and the checker's modules current.
