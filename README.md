# The Twigs catalog

Twigs anyone can add to a Twig Nest Hub: Credential types, Hop kinds, Tools, and Agents, as data. Nothing of the sort is built into the platform: every one a Hub knows comes from a Twig. A Hub lists this catalog on its Catalog page (`GET /v1/catalog`) and an admin installs a Twig from it in one click; no release of the platform is needed.

`core` is the bundle every Hub installs at start when it is missing: `kubernetes`, `http`, `github`, `gitlab`, `registry`, `network`, `ambient`, and `claude-code`. A Hub also installs, at start, any Twig of this catalog that provides a Credential type or Hop kind one of its Credentials or Hops uses but no installed Twig provides.

## Layout

- `twigs/<name>/twig.yaml`: one Twig (`api/schemas/manifests/twig.json`).
- `catalog.json`: the index (`api/schemas/manifests/catalog.json`), with each Twig file's URL and SHA-256. A Hub installs a Twig only when the file still has that digest.

## Adding a Twig

1. Write `twigs/<name>/twig.yaml`: `twig: twig-nest/twig/v1`, `name` (the directory's), `version`, `summary`, `publisher`, and what it provides. Name your own types and Tools: a name another installed Twig has is refused.
   - A Tool downloads a pinned release and checks its SHA-256 for every architecture, or installs a pinned Debian package from the base's snapshot (see `twigs/kubernetes` and `twigs/postgres`).
   - A Credential type has a preflight `authn` check that proves the Credential logs in, and a `permissions_hint` that says what read-only means for it.
   - A Credential type tells agents how to use it: `usage` is the one-line form (with the variables its slots and `env` set), and `guide` is a short Markdown how-to for the agent's brief: worked examples, how to pass a script, and which errors mean stop. The Runner adds on its own what the Credential sets and, for a write Credential, how `ops act` runs a command, so the guide does not repeat them. See `twigs/ssh`.
   - A Twig may `requires` other Twigs of this catalog (name, and optionally the lowest version); a Hub installs them first. A Twig of `requires` alone is a bundle that installs a set in one go: see `twigs/ops-essentials`.
2. Send a pull request to [shebang-labs/twigs](https://github.com/shebang-labs/twigs) with the file alone; `catalog.json` is rewritten when it lands. Raise `version` on every change: a Hub offers the newer version as an upgrade.

The checks live with the platform, not here: every Twig must validate against the Twig schema and install on a Hub. A maintainer runs them and lands the Twig in twig-nest's `catalog/`, which publishes this repository; a pull request here is merged that way, not by its button. In twig-nest: `go test ./catalog -update` rewrites `catalog.json`, `go test ./catalog ./internal/hub/api -run 'Catalog'` runs the checks, and `make publish-twigs` (or the `twigs` workflow on main) publishes.

## Trust

A Twig's commands run on Runners: a Tool's install step on the Forge, a type's checks and a Hop kind's commands in Jobs, under the Runner's isolation. Review a Twig as you would a Dockerfile. A Hub shows what a Twig provides and every command before an admin installs it.

## Your own catalog

Point a Hub at any `catalog.json` served over https, where it is deployed (`TWIG_NEST_CATALOG_URL`, or `hub.catalogURL` in its Helm chart) or in its console's Settings (Catalog URL, which wins): a fork of this directory works, and so does a mirror for a Hub that cannot reach GitHub. A Hub that cannot read its catalog at start keeps retrying until it can.
