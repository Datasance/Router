# iofog-router

Builds an image of the Apache Qpid Dispatch Router designed for use with Eclipse ioFog and Datasance Pot. The router can run in **Pot** mode (config from iofog agent) or **Kubernetes** mode (config from a volume-mounted file at `QDROUTERD_CONF`).

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SKUPPER_PLATFORM` | `pot` | Mode: `pot` or `iofog` (alias; config from iofog SDK) or `kubernetes` (config from file at `QDROUTERD_CONF`). |
| `QDROUTERD_CONF` | `/tmp/skrouterd.json` | Path to the router JSON config file. In Kubernetes mode the operator must volume-mount the router ConfigMap at this path. |
| `SSL_PROFILE_PATH` | `/etc/skupper-router-certs` | Directory under which SSL profile certs reside (e.g. `SSL_PROFILE_PATH/<profile-name>/ca.crt`, `tls.crt`, `tls.key`). Certs are mounted here in both K8s and Pot. |
| `EDGELET_MICROSERVICE_UID` | *(required in iofog/pot mode)* | Edgelet microservice identity used by the SDK client. |
| `SSL` | `true` | Enables HTTPS/WSS for ioFog Local API in Pot mode. |

## Pot mode and ioFog LocalAPI v3

In Pot mode, router reads config from ioFog Agent LocalAPI v3 using the SDK (`GET /v1/microservices/config`) and listens for update signals over control websocket (`/v1/microservices/control`).

To work correctly, the container must have ioFog service-account material mounted:

- token at `/var/run/secrets/edgelet.iofog.org/serviceaccount/token`
- CA at `/var/run/secrets/edgelet.iofog.org/serviceaccount/ca.crt`

In Kubernetes mode the router does not use the Kubernetes API; the operator is responsible for mounting the router config at `QDROUTERD_CONF`. Config file changes are watched and applied to the running router via qdr (same as Pot mode).
