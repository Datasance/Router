# Router

[![CI](https://github.com/eclipse-iofog/router/actions/workflows/ci.yml/badge.svg?branch=develop)](https://github.com/eclipse-iofog/router/actions/workflows/ci.yml)
[![Release](https://github.com/eclipse-iofog/router/actions/workflows/release.yml/badge.svg)](https://github.com/eclipse-iofog/router/actions/workflows/release.yml)
[![Go](https://img.shields.io/badge/Go-1.26.6-00ADD8?logo=go&logoColor=white)](https://go.dev/)

Go wrapper image for **[skupper-router](https://github.com/skupperproject/skupper-router)** used by **Eclipse ioFog** and **Datasance PoT** edge fleets. The wrapper supervises embedded **`skrouterd`** with config watch and AMQP hot reload.

| Component | Version |
|-----------|---------|
| Wrapper release | **v3.8.0** |
| Embedded skupper-router | **3.5.1** (compiled from upstream tag pin) |

Edgelet workload label: **`iofog-router`**.

## Container images

Identical git tree; registry and OCI labels differ by mirror CI variables only.

| Mirror | Image |
|--------|-------|
| Eclipse ioFog (upstream) | `ghcr.io/eclipse-iofog/router` |
| Datasance (PoT-facing) | `ghcr.io/datasance/router` |

Release tags: `:semver` (e.g. `:3.8.0`), `:latest`, and `:main` on a unified **4-platform** manifest (`linux/amd64`, `linux/arm64`, `linux/arm/v7`, `linux/riscv64`).

## Dual-mirror workflow

1. Develop on **`Datasance/router`** branch **`develop`** (or feature branches `router/<plan>-<slug>`).
2. Open PR to **`eclipse-iofog/router`** **`develop`**.
3. CI runs on `develop` (lint, test, docker smoke) — **no GHCR push** on branch builds.
4. After merge, tag identical **`v*`** releases on both remotes; **`release.yml`** publishes images.

See [CONTRIBUTING.md](CONTRIBUTING.md) for branch naming and contributor workflow.

## Platforms

| Platform | Dockerfile | Builder → runtime |
|----------|------------|-------------------|
| `linux/amd64`, `linux/arm64` | `Dockerfile` | UBI 9 → scratch |
| `linux/arm/v7`, `linux/riscv64` | `Dockerfile.edge` | Debian Trixie → scratch |

`s390x` and `ppc64le` are out of scope for the ioFog release manifest.

Local dev overlay: `Dockerfile.dev` (`quay.io/skupper/skupper-router:3.5.1` + wrapper only).

## Modes

The router runs in **Pot** mode (config from ioFog agent LocalAPI v3) or **Kubernetes** mode (config from a volume-mounted file at `QDROUTERD_CONF`). Set `SKUPPER_PLATFORM` accordingly.

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SKUPPER_PLATFORM` | `pot` | `pot` or `iofog` (alias; config from ioFog SDK) or `kubernetes` (config from file at `QDROUTERD_CONF`). |
| `QDROUTERD_CONF` | `/tmp/skrouterd.json` | Path to the router JSON config. In Kubernetes mode the operator must volume-mount the router ConfigMap at this path. |
| `QDROUTERD_HOME` | `/home/skrouterd` | Skupper home directory (set in image). |
| `SSL_PROFILE_PATH` | `/etc/skupper-router-certs` | Directory for SSL profile certs (`ca.crt`, `tls.crt`, `tls.key` per profile). Watched in both K8s and Pot modes. |
| `EDGELET_MICROSERVICE_UID` | *(required in pot/iofog mode)* | Edgelet microservice identity used by the SDK client. |
| `SSL` | `true` | Enables HTTPS/WSS for ioFog Local API in Pot/iofog mode. |

## Edgelet LocalAPI v1

In PoT/iofog mode, the wrapper reads config from Edgelet LocalAPI v1 using the SDK (`GET /v1/microservices/config`) and listens for update signals over the control websocket (`/v1/microservices/control`).

The container must have ioFog service-account material mounted:

- token at `/var/run/secrets/edgelet.iofog.org/serviceaccount/token`
- CA at `/var/run/secrets/edgelet.iofog.org/serviceaccount/ca.crt`

## Kubernetes mode

The wrapper does not use the Kubernetes API. The operator mounts router config at `QDROUTERD_CONF`. File changes are watched and applied to the running router via AMQP management (same hot-reload path as Pot mode).

## License

Eclipse Public License 2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).
