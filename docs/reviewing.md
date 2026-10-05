# Reviewing a Twig

A Twig's commands run on other teams' Runners, next to their credentials. Review a pull request as you would a Dockerfile someone asks you to run in production. CI has already checked the format and the rules a Nest enforces; this list is what a person checks.

## Every Twig

- [ ] It does one thing, under a name that says whose it is, and its `summary` says what an agent does with it.
- [ ] A changed Twig raised its `version` (CI checks), by a step that matches the change.
- [ ] `publisher`, `homepage`, and `license` are right.

## Tools

- [ ] Every URL is the project's own release (GitHub releases, the vendor's download host, the Debian snapshot), over HTTPS, for an exact version: no `latest`, no redirects to elsewhere.
- [ ] Every SHA-256 matches the published checksum file or a download you verified; `notes` says which. Recompute one if the project publishes none.
- [ ] amd64 and arm64 both resolve, and any other architecture fails loudly.
- [ ] The install writes only to `/usr/local`, `/opt/<tool>`, and `/tmp` (cleaned up), and runs no install scripts from package registries (`--ignore-scripts` for npm).
- [ ] Nothing phones home at runtime that the `notes` do not mention (update checks are turned off where possible).

## Credential types

- [ ] `preflight.authn` only proves the login: no write, and a timeout.
- [ ] `permissions_hint` gives the least privilege that reads, and says what counts as write.
- [ ] `authz` (when the provider can answer "may I?") asks about a write action, so a Nest can verify the credential is read-only.
- [ ] Secrets are delivered as files or variables the tool reads, never on a command line or in a URL.
- [ ] `usage` and `guide` are correct and never tell an agent to turn off a safety check (host key checking, TLS verification).

## Hop kinds

- [ ] `up` steps and daemons are the network's own client, started with the Hop's Secrets, and `down` logs out.
- [ ] `requires_capabilities` asks for `net_admin` only when a kernel-mode network needs it.

## Agents

- [ ] The binary comes from the Twig's Tool, and the arguments start the agent on ACP without a shell.
- [ ] `key_env` and `pass_env` list only what the agent needs.

When in doubt, ask in the pull request. Merging squashes the commits, and the index workflow publishes the Twig to every Nest within minutes.
