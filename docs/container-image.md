# Container image

Multi-stage [`Dockerfile`](https://github.com/platform-boutique/app-shipping-service/tree/main/Dockerfile):

| Stage | Base | What happens |
|---|---|---|
| builder | `golang:1.26-alpine` | `go mod download`, then `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" ./cmd/server` |
| runtime | `alpine:3.21` | single binary at `/app/shipping-service` |

- **Entrypoint:** `/app/shipping-service`. `GOTRACEBACK=single`.
- **User: root** — no `USER` in the Dockerfile; the image runs as UID 0 (verified). Set a non-root `runAsUser` (e.g. `10001`) in Kubernetes; the binary needs no privileges.
- **Architecture: `amd64` only** (`GOARCH=amd64` hard-coded) — `exec format error` on `arm64` nodes.
- **EXPOSE:** `50051`, `9090`. **`HEALTHCHECK`:** exec form `["/app/shipping-service", "healthcheck"]` (gRPC `grpc.health.v1` Check on `127.0.0.1:${PORT:-50051}`, 3 s deadline, no shell tools).
- **Image size:** ~10 MB. No `.dockerignore`, so `.git` is sent in the build context.
