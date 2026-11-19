# Changelog


## [v3.9.5] - 2026-11-19

### Wrapper release

- **v3.9.5** — align with the component release train. **Go 1.26.11** minimum (was 1.26.10; see `go.mod`). CI, release, and the govulncheck workflow use the same toolchain. No wrapper behavior change.

### SDK and LocalAPI

- **iofog-go-sdk v3.9.5** (`github.com/eclipse-iofog/iofog-go-sdk/v3`) — upgrade from v3.9.4. Same Go 1.26.11 pin; no transitive module change (`golang.org/x/sys` stays **v0.48.0**).

### Embedded skupper-router

- Pin and compile upstream **skupper-router 3.5.4** (was 3.5.3; separate from wrapper semver). Upstream delta since 3.5.3: router metrics attribute-label alignment (#1884, #1885) and the Containerfile/CI pin to Qpid Proton **0.41.1** (#1887). Local builds set `PROTON_VERSION=0.41.1` (was commit `e5d5c2b`). No config, `$management`, launch, or image-layout change.

### CI and release

- **`ci.yml`**, **`release.yml`**, **`govulncheck.yml`**: Go **1.26.11** in workflow env.
- **`Dockerfile`**, **`Dockerfile.dev`**, **`Dockerfile.edge`**: `golang:1.26.11-alpine` builder image (digest-pinned in prod/edge).
- **`Dockerfile`**: compile skupper-router **3.5.4** with Proton **0.41.1**; refresh digest pins for `ubi9/ubi-minimal` and `ubi9/ubi` (builder, packager, and tz stages).
- **`Dockerfile.edge`**: compile skupper-router **3.5.4** with Proton **0.41.1**; refresh digest pins for `debian:trixie` (builder and packager) and `debian:trixie-slim`.
- **`Dockerfile.dev`**: base image `quay.io/skupper/skupper-router:3.5.4`.


## [v3.9.4] - 2026-10-29

### Wrapper release

- **v3.9.4** — align with the component release train. No wrapper or Go version change (toolchain stays **1.26.10**).

### SDK and LocalAPI

- **iofog-go-sdk v3.9.4** (`github.com/eclipse-iofog/iofog-go-sdk/v3`) — upgrade from v3.9.3. Same Go 1.26.10 pin; no transitive module change (`golang.org/x/sys` stays **v0.48.0**).

### Embedded skupper-router

- Pin and compile upstream **skupper-router 3.5.3** (unchanged from v3.9.3; separate from wrapper semver).

### CI and release

- **`Dockerfile`**: refresh digest pins for `ubi9/ubi-minimal` and `ubi9/ubi` (builder, packager, and tz stages), and for `golang:1.26.10-alpine`.
- **`Dockerfile.edge`**: refresh digest pins for `debian:trixie` (builder and packager), `golang:1.26.10-alpine`, and `debian:trixie-slim`.

## [v3.9.3] - 2026-10-26

### Wrapper release

- **v3.9.3** — align with the component release train. No wrapper or Go version change (toolchain stays **1.26.10**).

### SDK and LocalAPI

- **iofog-go-sdk v3.9.3** (`github.com/eclipse-iofog/iofog-go-sdk/v3`) — upgrade from v3.9.2. Same Go 1.26.10 pin; no transitive module change (`golang.org/x/sys` stays **v0.48.0**).

### Embedded skupper-router

- Pin and compile upstream **skupper-router 3.5.3** (unchanged from v3.9.2; separate from wrapper semver).

### CI and release

- **`Dockerfile`**: refresh digest pins for `ubi9/ubi-minimal` and `ubi9/ubi` (builder, packager, and tz stages), and for `golang:1.26.10-alpine`.
- **`Dockerfile.edge`**: refresh the `golang:1.26.10-alpine` digest pin.

## [v3.9.2] - 2026-10-22

### Wrapper release

- **v3.9.2** — security/maintenance patch. **Go 1.26.10** minimum (was 1.26.9; see `go.mod`). CI, release, and the govulncheck workflow use the same toolchain. Clears [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809) (`crypto/x509` distrust of compromised intermediate certificates). No wrapper behavior change.

### SDK and LocalAPI

- **iofog-go-sdk v3.9.2** (`github.com/eclipse-iofog/iofog-go-sdk/v3`) — upgrade from v3.9.1. Same Go 1.26.10 pin; no transitive module change (`golang.org/x/sys` stays **v0.48.0**).

### Embedded skupper-router

- Pin and compile upstream **skupper-router 3.5.3** (unchanged from v3.9.1; separate from wrapper semver).

### CI and release

- **`ci.yml`**, **`release.yml`**, **`govulncheck.yml`**: Go **1.26.10** in workflow env.
- **`Dockerfile`**, **`Dockerfile.dev`**, **`Dockerfile.edge`**: `golang:1.26.10-alpine` builder image (digest-pinned in prod/edge).
- **`Dockerfile`**: refresh digest pins for `ubi9/ubi-minimal` and `ubi9/ubi` (builder, packager, and tz stages).
- **`Dockerfile.edge`**: refresh digest pins for `debian:trixie` and `debian:trixie-slim`.

## [v3.9.1] - 2026-10-10

### Wrapper release

- **v3.9.1** — security/maintenance patch. Go toolchain **1.26.9** (was 1.26.6) picks up the 8 October 2026 stdlib fixes reported by [govulncheck on develop](https://github.com/eclipse-iofog/router/actions/runs/38041668413/job/114182978972). Reachable `net/http` symbols are `ProxyFromEnvironment`, `Header.Get`, `Header.Set` (Edgelet control websocket) and `NewRequest`, `Client.Do` (Edgelet config fetch): GO-2026-6603, GO-2026-6610, GO-2026-6611, GO-2026-6612, GO-2026-6617. No wrapper behavior change.

### SDK and LocalAPI

- **iofog-go-sdk v3.9.1** (`github.com/eclipse-iofog/iofog-go-sdk/v3`) — upgrade from v3.9.0. Same Go 1.26.9 pin; no `pkg/microservices` API change. Transitive `golang.org/x/sys` **v0.48.0** (was v0.46.0).

### Embedded skupper-router

- Pin and compile upstream **skupper-router 3.5.3** (unchanged from v3.9.0; separate from wrapper semver).

### CI and release

- **`ci.yml`**, **`release.yml`**, **`govulncheck.yml`**: Go **1.26.9** in workflow env.
- **`govulncheck.yml`**: schedule is daily (`0 0 * * *`), was weekly.
- **`Dockerfile`**, **`Dockerfile.dev`**, **`Dockerfile.edge`**: `golang:1.26.9-alpine` builder image (digest-pinned in prod/edge).
- **`Dockerfile`**: refresh digest pins for `ubi9/ubi-minimal` and `ubi9/ubi`.
- **`Dockerfile.edge`**: refresh digest pins for `debian:trixie` and `debian:trixie-slim`.

## [v3.9.0] - 2026-09-26

### Wrapper release

- **v3.9.0** — align with the **controller 3.9** release train.

### SDK and LocalAPI

- **iofog-go-sdk v3.9.0** (`github.com/eclipse-iofog/iofog-go-sdk/v3`) — upgrade from v3.9.0-rc.1.

### Embedded skupper-router

- Pin and compile upstream **skupper-router 3.5.3** (was 3.5.2; separate from wrapper semver). Patch release: inter-router parse-depth hardening (#1879), allow streaming links on inter-edge connections (#1878), Proton 0.41.1 CI/compat (#1881). No config, `$management`, launch, or image-layout change.

### CI and release

- **`Dockerfile`**: compile skupper-router **3.5.3**; refresh digest pins for `ubi9/ubi-minimal` and `ubi9/ubi`.
- **`Dockerfile.edge`**: compile skupper-router **3.5.3**; refresh digest pins for `debian:trixie` and `debian:trixie-slim`.
- **`Dockerfile.dev`**: base image `quay.io/skupper/skupper-router:3.5.3`.

## [v3.9.0-rc.1] - 2026-09-17

### Wrapper release

- **v3.9.0** — align with the **controller 3.9** release train. No wrapper, Go, or skupper-router version change.

### SDK and LocalAPI

- **iofog-go-sdk v3.9.0-rc.1** (`github.com/eclipse-iofog/iofog-go-sdk/v3`) — upgrade from v3.8.3-rc.1.

### Embedded skupper-router

- Pin and compile upstream **skupper-router 3.5.2** (unchanged from v3.8.3; separate from wrapper semver).

### CI and release

- **`Dockerfile`**: refresh digest pins for `ubi9/ubi-minimal` and `ubi9/ubi`.

## [v3.8.3-rc.2] - 2026-08-26

### Wrapper release

- **v3.8.3-rc.2** — security/maintenance RC; bump indirect **`github.com/gorilla/websocket` v1.5.3** (was v1.5.0) to fix GO-2026-6278 (weak PRNG for WebSocket mask key). No wrapper, Go, or SDK version change.

### CI and release

- **`Dockerfile`**: refresh digest pins for `ubi9/ubi-minimal` and `ubi9/ubi`.
- **`Dockerfile.edge`**: refresh digest pins for `debian:trixie` and `debian:trixie-slim`.

## [v3.8.3-rc.1] - 2026-08-20

### Wrapper release

- **v3.8.3** — security/maintenance patch; Go toolchain **1.26.6** (was 1.26.5) to pick up stdlib fixes (GO-2026-6218, GO-2026-6090, GO-2026-5972, GO-2026-5026). No wrapper change.
- Bump iofog-go-sdk to **iofog-go-sdk v3.8.3-rc.1**

### Embedded skupper-router

- Pin and compile upstream **skupper-router 3.5.2** (was 3.5.1; separate from wrapper semver).

### CI and release

- **`ci.yml`**, **`release.yml`**, **`govulncheck.yml`**: Go **1.26.6** in workflow env.
- **`Dockerfile`**, **`Dockerfile.dev`**, **`Dockerfile.edge`**: `golang:1.26.6-alpine` builder image (digest-pinned in prod/edge).
- **`Dockerfile`**: refresh digest pins for `ubi9/ubi-minimal` and `ubi9/ubi`; compile skupper-router **3.5.2**.
- **`Dockerfile.edge`**: refresh digest pins for `debian:trixie` and `debian:trixie-slim`; compile skupper-router **3.5.2**.

## [v3.8.2] - 2026-07-25

### Wrapper release

- **v3.8.2** — maintenance patch; refresh digest-pinned base images for UBI and Debian edge builds. No wrapper, Go, or SDK version changes.

### Embedded skupper-router

- Pin and compile upstream **skupper-router 3.5.1** (unchanged from v3.8.1; separate from wrapper semver).

### CI and release

- **`ci.yml`**: remove Docker GHA cache from the `docker-smoke` job (`cache-from` / `cache-to`).
- **`Dockerfile`**: refresh digest pins for `ubi9/ubi-minimal` and `ubi9/ubi`.
- **`Dockerfile.edge`**: refresh digest pins for `debian:trixie` and `debian:trixie-slim`.

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
