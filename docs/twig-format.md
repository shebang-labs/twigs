# The Twig format

A Twig is a YAML file at `twigs/<name>/twig.yaml`. [`schema/twig.json`](../schema/twig.json) is its JSON Schema (draft 2020-12), with the schemas of the items it provides beside it; this page explains the fields, and the schemas have the exact rules and every description.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/twig-nest/twigs/main/schema/twig.json
twig: twig-nest/twig/v1      # the format, always this
name: mytool                 # the directory's name
version: 1.0.0               # raise it on every change
summary: One line on what it adds
publisher: Who maintains it
homepage: https://...        # optional
license: Apache-2.0          # optional: this file's license; Tools' notes name theirs
tags: [database]             # optional, for search
requires:                    # optional: other Twigs, installed first
- name: kubernetes
  version: 1.0.0             # optional: the lowest that will do
authenticator_types: []      # optional: Credential types
hop_kinds: []                # optional: Hop kinds
tools: []                    # optional: Tools
agents: []                   # optional: Agents
```

A Twig with only `requires` is a **bundle**: installing it installs the set ([`core`](../twigs/core/twig.yaml), [`ops-essentials`](../twigs/ops-essentials/twig.yaml)).

## Tools

A Tool is a CLI that Forge, the Nest's image builder, installs into the images agents run in ([schema](../schema/tool.json)).

| Field | Required | What it is |
|---|---|---|
| `id` | yes | The Tool's name, as Images list it (`kubectl`). |
| `version` | yes | The version the fragment installs. |
| `install` | yes | Dockerfile instructions run after the Nest's `FROM`, as root: `RUN`, `ARG`, `ENV`, `WORKDIR`, and `COPY --from=<image>@sha256:<digest>`. `ARG TARGETARCH` is set to `amd64` or `arm64`. |
| `provides` | yes | Each binary it puts on `PATH`, with the `version_command` that prints its version. |
| `summary` | | One line. |
| `platforms` | | The platforms it supports; both by default. |
| `notes` | | Where the checksums come from, the license, anything a reviewer needs. |

## Credential types

A Credential type says how a kind of credential reaches a Job and how to prove it works ([schema](../schema/credential-type.json)). An admin creates Credentials of the type; their values are Secrets the Nest keeps and sends with each Job.

| Field | Required | What it is |
|---|---|---|
| `authenticator_type` | yes | Its name, `<provider>.<kind>` (`mysql.password`). |
| `version`, `provider`, `summary` | yes | Raise `version` on every change a Credential could notice. |
| `interactive` | yes | `true` only when a person must act (a browser login); Jobs cannot use such a type. |
| `params` | yes | A JSON Schema for the Credential's non-secret parameters (a host, a port, a region). |
| `permissions_hint` | yes | What an admin grants so the credential only reads, and which Secrets to fill. |
| `secrets` | | The slots filled with Secrets, each delivered `as: file` (at `path`, with its path in `env`) or `as: env`. |
| `env` | | Non-secret variables, as templates of `{params.x}`. |
| `requires_tools`, `requires_network` | | The binaries its checks need, and the hosts they reach. |
| `preflight` | | `authn`: a command that proves the credential logs in, and what its result must be (`expect`: an exit code or a JSON path). `authz`: how to ask whether an action is allowed, so read-only can be verified. |
| `usage` | | The one-line form an agent runs, as a template. |
| `guide` | | A short Markdown how-to for the agent: worked examples, and which errors mean stop. |
| `git` | | For a git host's token: the Runner sets git's credential helper for the host. |
| `variants` | | Fields that differ by the value of one param (see [`ambient`](../twigs/ambient/twig.yaml)). |

Commands are templates: `{params.x}` for a param, `{secrets.slot}` for a file slot's path, and `{credential.name}` for the Credential's name. They run without a shell, split into words; the checks refuse a `{params.x}` or `{secrets.x}` the type does not declare.

## Hop kinds

A Hop kind is a step a Job's route to a Target crosses: a port that must answer, a network to join ([schema](../schema/hop-kind.json)).

| Field | Required | What it is |
|---|---|---|
| `type`, `version`, `summary` | yes | Its name, `<area>.<kind>` (`net.tailscale`). |
| `params` | yes | A JSON Schema for the Hop's parameters. |
| `preflight` | yes | The check that the Hop works, and its expected result. |
| `remedy` | yes | What to do when the check fails. |
| `interactive` | yes | `true` when a person must cross it. |
| `secrets`, `files`, `env` | | What a joining kind needs and exports (a proxy address). |
| `up`, `ready`, `down` | | For a kind that joins a network: the steps (and daemons) that join it, the check run until it is ready, and the commands run at the end. |
| `requires_tools`, `requires_network`, `requires_capabilities` | | The binaries, destinations, and Runner capabilities (`net_admin`) it needs. |
| `variants` | | Fields that differ by the value of one param, as for Credential types. |

Its templates use `{params.x}`, `{secrets.slot}`, and `{file.name}` (the path of one of its `files`), and, for the Hop itself, `{hop.dir}` (its directory), `{hop.port1}` and `{hop.port2}` (two free local ports for its proxies), and `{hop.name}`.

## Agents

An Agent is an agent CLI the Runner starts for an agent Job, speaking the [Agent Client Protocol](https://agentclientprotocol.com) on stdio ([schema](../schema/agent.json)).

| Field | Required | What it is |
|---|---|---|
| `id` | yes | Its name, as Profiles choose it. |
| `binary` | yes | The executable the Runner starts. |
| `args` | yes | The arguments that make it speak ACP; templates may use `{model}`, `{budget_usd}`, `{system_prompt}`, and `{job.dir}`. |
| `tool` | | The Tool that installs the binary, in this Twig or one it requires. |
| `key_env`, `pass_env`, `env` | | The variables that hold its model key, the ones it may read from the Runner, and fixed ones. |
| `acp` | | What the Runner tells it beyond ACP's defaults: the authentication method and session options. |
| `resume` | | Where it keeps sessions, so a Job picks up after a Runner restart. |
| `context_files`, `version_command`, `summary` | | Files it reads besides `AGENTS.md`, how to print its version, one line. |

## The index

`catalog.json` lists every Twig with its summary, what it provides, what it requires, the URL of its file, and the file's SHA-256 ([schema](../schema/catalog.json)). A Nest reads it, and installs a Twig only when the file still has that digest. `make index` writes it, and the index workflow runs it after every merge.
