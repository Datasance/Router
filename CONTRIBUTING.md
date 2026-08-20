# Contributing

Thank you for contributing to the ioFog / Datasance **router** wrapper image.

## Repositories

| Role | GitHub | Container registry |
|------|--------|------------------|
| Upstream (canonical module path) | [eclipse-iofog/router](https://github.com/eclipse-iofog/router) | `ghcr.io/eclipse-iofog/router` |
| Mirror (primary development remote) | [Datasance/router](https://github.com/Datasance/router) | `ghcr.io/datasance/router` |

The git tree is identical on both remotes. Product flavor (registry URL, OCI labels) is selected by CI repository variables — not by forked application code.

## Branch workflow

| Branch | Purpose |
|--------|---------|
| **`develop`** | Integration branch on **both** remotes |
| **`router/<plan>-<slug>`** | Feature / plan branches (e.g. `router/07-docs`) |

Typical flow:

1. Branch from **`develop`** on **`Datasance/router`**.
2. Open a pull request to **`eclipse-iofog/router`** **`develop`**.
3. Ensure CI is green (lint, test, docker smoke on `develop` pushes and PRs).
4. After merge, release maintainers tag identical **`v*`** semver on both remotes; **`release.yml`** publishes GHCR images.

Do **not** use the legacy **`iofog/merge`** branch — it is abandoned in favor of the develop workflow above.

## Development setup

- **Go 1.26.6** (see `go.mod`).
- `make test`, `make fmt-check`, `make security-code` before pushing.
- Local wrapper overlay: `Dockerfile.dev` (upstream `quay.io/skupper/skupper-router:3.5.2` image).

Module import path is always **`github.com/eclipse-iofog/router`**, even when cloning the Datasance mirror.

## Pull requests

- Target **`develop`**, not `main`.
- Keep changes focused; one active implementation plan per branch when following the modernization wave.
- Do not reintroduce: per-file copyright headers, `push.yaml`, CI push on every branch push, `secrets.PAT` for routine CI, or **`router-adaptor`** publish paths.

## Releases

- Wrapper git tags: **`v3.8.0`**, **`v3.8.0-1`**, etc.
- Embedded skupper-router version (**3.5.1**) is documented separately from wrapper semver.
- Images publish only on **`v*` tag push** — not on ordinary `develop` commits.

## Questions

Open an issue on the mirror you are developing against, or reach out to the ioFog / Datasance maintainers for release coordination between remotes.
