# Contributing

Thanks for adding to the catalog. A Twig is one YAML file, and adding one is a single pull request: no platform release is needed, and every Twig Nest can install it once it is merged.

Please follow the [code of conduct](CODE_OF_CONDUCT.md). To report a security problem, read [SECURITY.md](SECURITY.md) instead of opening an issue.

## What you need

- [Go](https://go.dev/dl/), the version `go.mod` names. The checker is a small Go program; `make` runs it with `go run`, so there is nothing to install.
- Optional: `yamllint` (`pip install yamllint==1.37.1`) for `make lint`.
- Optional but recommended: an editor with the YAML language server (VS Code's [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml), JetBrains IDEs, Neovim). Every `twig.yaml` starts with a `yaml-language-server` comment that points it at [`schema/`](schema), so it completes fields and underlines mistakes as you type.

## Add a Twig

1. Fork the repository and start the file:

   ```sh
   make new NAME=mytool
   ```

   This writes `twigs/mytool/twig.yaml` from a template with one Tool. The directory's name is the Twig's name: lower case letters, digits, and `-`.
2. Fill it in. [docs/twig-format.md](docs/twig-format.md) describes every field, and the existing Twigs are worked examples:
   - a CLI: [`terraform`](twigs/terraform/twig.yaml), or [`postgres`](twigs/postgres/twig.yaml) for a Debian package;
   - a Credential type with read-only checks: [`ssh`](twigs/ssh/twig.yaml), [`mysql`](twigs/mysql/twig.yaml);
   - a Hop kind that joins a network: [`tailscale`](twigs/tailscale/twig.yaml);
   - an agent: [`opencode`](twigs/opencode/twig.yaml);
   - a bundle of other Twigs: [`ops-essentials`](twigs/ops-essentials/twig.yaml).
3. Check it:

   ```sh
   make check                        # every Twig, and the checker's tests
   make check BASE=upstream/main     # also: a changed Twig raised its version (in a fork: git remote add upstream https://github.com/twig-nest/twigs.git && git fetch upstream)
   make lint                         # yamllint and actionlint
   ```

   CI runs `make check` against the pull request's base branch, and `make lint`.

4. Open a pull request. CI runs the same checks and annotates any problem on the file. Leave `catalog.json` and the README's table alone: after the merge, the [index workflow](.github/workflows/index.yml) rewrites both.

To change a Twig, edit its file, raise its `version` (a fix or a comment is a patch release, a new Tool version or field a minor one, `-rc.1` style pre-releases are fine), and follow steps 3 and 4. Any change to the file needs a new version: a Nest installs a file by its digest, so a changed file at the same version would never reach anyone.

## What the checks check

`make check` runs `twigs check`. Each problem names the file and, when it can, the [JSON pointer](https://datatracker.ietf.org/doc/html/rfc6901) of the field.

| Check | Why |
|---|---|
| The file is valid YAML and matches [`schema/twig.json`](schema/twig.json), which references the schemas of each kind | A Nest validates the same schemas and refuses anything else. |
| `twigs/` holds only `twigs/<name>/twig.yaml`, `name` is the directory's name, in lower case letters, digits, and `-` | The index and a Nest's install use it. |
| A Credential type, Hop kind, Tool, or Agent name is provided once in the whole catalog | A Nest refuses a Twig whose names another installed Twig has. |
| Every `requires` names a Twig here, at that version or newer, and no Twig requires itself | A Nest installs requirements from this catalog first. |
| A Tool's `install` holds only `RUN`, `ARG`, `ENV`, `WORKDIR`, and `COPY --from=<image>@sha256:<digest>`, no build argument that looks like a secret, no template placeholders, and lists each binary once | The Nest's image renderer owns `FROM`, `USER`, and `ENTRYPOINT`, and builds never carry secrets. |
| An Agent's `tool` is in its Twig or one it requires, and provides the Agent's `binary`; its `args`, `env`, and `acp` use only `{model}`, `{budget_usd}`, `{system_prompt}`, and `{job.dir}` | The Runner starts the binary from that Tool, and sets only those placeholders. |
| A Credential type's or Hop kind's `{params.x}`, `{secrets.x}`, and `{file.x}` name a param, secret slot, or file it declares | A misspelled placeholder reaches the Runner unexpanded. |
| A changed Twig raised its `version`, and `catalog.json` is as the base branch has it (pull requests) | A Nest offers a newer version as an upgrade, and installs a file only at the digest `catalog.json` names, which the index workflow writes. |
| `core` exists and the index renders | Every Nest installs `core` at start, from `catalog.json`. |

`make lint` runs yamllint on every YAML file and actionlint on the workflows.

## Conventions

- **Pin and check every download.** A Tool installs one exact version for `amd64` and `arm64`, checks each file's SHA-256 before using it, and fails on any other architecture. A Debian package names its exact version as Debian 13 (trixie) has it: Nests build Images on a Debian 13 base whose package sources are pinned to a [snapshot.debian.org](https://snapshot.debian.org) date, so `apt-get install pkg=version` resolves the same everywhere. `notes` says where the checksums come from and the license of what is installed.
- **Read-only first.** A Credential type's `preflight.authn` proves the credential logs in; its `permissions_hint` tells an admin what to grant so the credential can only read; where the provider can answer "may I?", say how in `authz` so a Nest can verify it.
- **Write for the agent.** A Credential type's `usage` is the one-line form an agent runs; its `guide` is a short Markdown how-to with worked examples and the errors that mean stop.
- **Name your own things.** Use names that say whose they are (`mysql.password`, `net.tailscale`, `kubectl`), not generic ones another Twig would need.
- **Keep it small.** One tool, service, or network per Twig; a bundle Twig of `requires` alone groups them.

## Where to start

Issues labeled [good first issue](https://github.com/twig-nest/twigs/labels/good%20first%20issue) and [new twig](https://github.com/twig-nest/twigs/labels/new%20twig) are a good start: a requested CLI is usually one Tool, and `terraform` or `vault` show the whole pattern.

## How changes are reviewed

A maintainer reviews every pull request as they would a Dockerfile, with [docs/reviewing.md](docs/reviewing.md): the sources, the checksums, and every command. Expect questions about where a checksum comes from; that is the job, not doubt about you.

## Working on the checker

The checker lives in [`cmd/twigs`](cmd/twigs) and [`internal/catalog`](internal/catalog); `make test` runs its tests. The schemas in [`schema/`](schema) are the platform's own, published here: changes to them come from Twig Nest's maintainers, so open an issue rather than editing them.

## License

By contributing you agree that your contribution is licensed under the [Apache License 2.0](LICENSE), as the repository is.
