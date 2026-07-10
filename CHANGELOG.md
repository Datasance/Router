# Changelog

## [v3.8.1] - 2026-07-10

### Wrapper release

- **v3.8.1** — patch release aligning with **iofog-go-sdk v3.8.1**.
- Go toolchain **1.26.5** (was 1.26.4).

### SDK and LocalAPI

- **iofog-go-sdk v3.8.1** (`github.com/eclipse-iofog/iofog-go-sdk/v3`) — upgrade from v3.8.0-rc.2.

### CI and release

- **`ci.yml`**, **`release.yml`**, **`govulncheck.yml`**: Go **1.26.5** in workflow env.
- **`Dockerfile`**, **`Dockerfile.dev`**, **`Dockerfile.edge`**: `golang:1.26.5-alpine` builder image (digest-pinned in prod/edge).

## [v3.8.0] - 2026-06-17

### Wrapper release

- **v3.8.0** — dual-mirror modernization of the ioFog skupper-router wrapper image.
- Go wrapper binary **`router`** at `/home/skrouterd/bin/router`; supervises **`skrouterd`** via `launch.sh`.
- Canonical Go module: **`github.com/eclipse-iofog/router`** on both remotes.
- Go toolchain **1.26.4**.

### Embedded skupper-router

- Pin and compile upstream **skupper-router 3.5.1** (separate from wrapper semver).
- Production images: UBI 9 builder (amd64/arm64) and Debian Trixie builder (arm/v7, riscv64) → **scratch** runtime with identical `/image` layout.
- Dev overlay: `Dockerfile.dev` on `quay.io/skupper/skupper-router:3.5.1`.

### SDK and LocalAPI

- **iofog-go-sdk v3.8.0-beta.4** (`github.com/eclipse-iofog/iofog-go-sdk/v3`) — interim dependency until eclipse-iofog SDK migration.
- Pot/iofog mode: ioFog Agent **LocalAPI v3** config fetch and control websocket updates.
- **`SKUPPER_PLATFORM`**: `pot`, `iofog` (alias), `kubernetes`.

### CI and release

- **`ci.yml`** on **`develop`**: lint, test, security analysis, 4-platform docker smoke — **no GHCR push** on branch builds.
- **`release.yml`** on **`v*` tags only**: quality gates, UBI + edge image publish, unified 4-platform manifest (`:semver`, `:latest`, `:main`).
- **`govulncheck.yml`**: scheduled vulnerability scanning.
- Mirror-specific registry via **`set-build-env`** and repository variables; **`GITHUB_TOKEN`** for GHCR (no routine `secrets.PAT`).
- SHA-pinned GitHub Actions and base images.

### Multi-arch

- **amd64**, **arm64** (`Dockerfile`), **arm/v7**, **riscv64** (`Dockerfile.edge`) in one manifest.
- **s390x** and **ppc64le** excluded from ioFog release manifest.

### Deprecations and removals

- Abandon **`iofog/merge`** long-lived branch in favor of **`develop`** → **`develop`** PR workflow between mirrors.
- Remove legacy **`push.yaml`** CI (replaced by `ci.yml` + `release.yml`).
- Remove Azure Pipelines workflow; GitHub Actions only.
- **`router-adaptor`** image out of scope (not published).
- Per-file copyright headers replaced by root **`NOTICE`** + **`LICENSE`**.

## [v3.0.0] - 10 May 2022

* Updated iofog-go-sdk to v3.0.0

## [v3.0.0-beta1] - 13 August 2021

* Pin Docker image to Go 1.16.7

## [v3.0.0-alpha1] - 24 March 2021

* No changes, releasing to synchronize with other projects

## [2.0.1] - 2020-08-28

* Add qdstat to Docker image

## [2.0.0] - 2020-08-05

* Initial release
* Add support for deployment in ioFog ECN
