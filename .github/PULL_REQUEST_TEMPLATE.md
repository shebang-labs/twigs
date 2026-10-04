<!-- Thanks for the Twig! CONTRIBUTING.md has the details behind each item. -->

### What it adds

<!-- One or two lines: the tool, credential, network, or agent, and who needs it. -->

### Checklist

- [ ] `make check` passes (CI runs it too, and annotates any problem on the file).
- [ ] Every download is pinned to a version and checked against its SHA-256, for amd64 and arm64, and `notes` says where the checksums come from.
- [ ] A changed Twig raises its `version`; a new one starts at `1.0.0`.
- [ ] A Credential type's checks only read: its `authn` check proves the credential logs in, and its `permissions_hint` says what read-only means.
- [ ] I did not edit `catalog.json` or the README's table: a bot rewrites them after the merge.

### How I tested it

<!-- On a Hub, if you have one: installed it (dry run is fine), built an Image with its Tools, ran preflight with its Credential type. -->
