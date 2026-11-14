# shipping-service

A gRPC service that provides shipping quotes and "ships" orders for the platform-demo e-commerce platform. It is a simulation: quotes come from a made-up tariff by destination zone and item count, and `ShipOrder` validates the address and fabricates a tracking ID. It is stateless, has no downstream dependencies, and is called by `checkout-service` (and by `frontend` for the cart-page shipping estimate).

Part of the [platform-demo](https://github.com/platform-boutique) platform — microservices, observability (LGTM stack), GitOps (Argo CD), policy enforcement (Kyverno), infrastructure provisioning (Terraform) and an internal developer portal (Backstage). See the master `README.md` in the platform-demo root for the whole-system view.

## At a glance

| | |
|---|---|
| **Language / runtime** | Go 1.25 (static binary, `CGO_ENABLED=0`) |
| **Image** | `mladenovskistefan/shipping-service` |
| **Protocol / port** | gRPC (HTTP/2, plaintext) on `50051` |
| **Metrics** | Prometheus `/metrics` on `9090` (scraped by Alloy) and pushed over OTLP/gRPC (registry bridged into OpenTelemetry); `build_info` gauge |
| **Health** | Standard `grpc.health.v1.Health/Check` on `50051` — `SERVING`, `NOT_SERVING` while draining after SIGTERM |
| **State** | None |
| **Depends on** | Nothing (optional: OTLP collector, Pyroscope) |
| **Called by** | `checkout-service` (`GetQuote`, `ShipOrder`), `frontend` (`GetQuote`) |
| **gRPC reflection** | Enabled — `grpcurl` works without `-proto` |

## API

Package `hipstershop`, service `ShippingService` ([`proto/shipping.proto`](proto/shipping.proto)).

| Method | Request | Response | Behaviour |
|---|---|---|---|
| `GetQuote` | `{ Address address, repeated CartItem items }` | `{ Money cost_usd, int32 estimated_days, string zone }` | Prices by **destination zone and item count** (see the tariff below). A **nil address or blank country uses the default zone (`domestic`)** (the cart-page estimate sends none); a country that is given is checked exactly like `ShipOrder` does (same function, `ResolveCountry`) and an unrecognised one → `INVALID_ARGUMENT` (`invalid address: country "X" is not an ISO 3166-1 alpha-2 code or a known country name (use e.g. "United States" or "US")`), so checkout fails at quote time, before the card is charged. 0 items → $0.00 and `estimated_days` 0. Always USD |
| `ShipOrder` | `{ Address address, repeated CartItem items }` | `{ string tracking_id }` | Validates the address (rules below); a nil or invalid address → `INVALID_ARGUMENT` with a message listing every problem. Otherwise returns a **deterministic** tracking ID like `LN-64645-325830843`: letters and digits come from the SHA-256 of the order content (street, city, state, ISO country code, zip, and the items as product id and summed quantity, sorted), the numbers in front are the length of that text and half of it. A retried `ShipOrder` for the same order returns the same ID (two identical orders to the same address share one). Nothing is dispatched or stored |

`Money` = `{ currency_code, int64 units, int32 nanos }`. `frontend` calls `GetQuote` with a nil address and the cart items for the cart-page estimate; checkout passes the real address. `estimated_days` (field 2) and `zone` (field 3) were added to `GetQuoteResponse`; existing fields and numbers are unchanged, so old clients keep working.

### Tariff (simulated)

The request carries **no weights** (only `product_id` and `quantity`), so the price depends on the zone and the summed item quantity only (non-positive quantities are ignored). Price = base (first item) + per extra item × (items − 1).

| Zone (`zone`) | Countries | Base | Per extra item | `estimated_days` |
|---|---|---|---|---|
| `domestic` | US | $8.99 | $1.50 | 3 |
| `north_america` | CA, MX | $14.99 | $2.50 | 6 |
| `europe` | EU, EEA, GB, CH, the European microstates, AL, BA, ME, MK, RS, MD, UA, BY and the Crown dependencies/Nordic territories (AX, FO, GG, GI, IM, JE, SJ) | $19.99 | $3.00 | 8 |
| `rest_of_world` | everything else (including US territories such as PR, and RU, TR) | $29.99 | $4.50 | 14 |

One item to the US still costs $8.99, the former flat rate.

### Address validation (`ShipOrder`; the country check also in `GetQuote`)

Deliberately lenient:

- `street_address`, `city` and `country` must be non-blank; `state` is not checked.
- `country` must be an ISO 3166-1 alpha-2 code (any case, all 249 assigned codes) or an English country name or common alias (all 249 ISO names plus Kosovo/`XK`, e.g. `United States`, `USA`, `U.S.A.`, `UK`, `Great Britain`, `Deutschland`, `Thailand`, `Taiwan`, `Vietnam`, `Hong Kong`, `Ivory Coast`/`Côte d'Ivoire`; case, dots, hyphens, commas and accents are ignored), normalised to the ISO code. `GetQuote` applies only this country check (not street, city or zip) and only when a non-blank country is given. The frontend form sends `United States` by default.
- `zip_code` is an `int32`, so alphanumeric postcodes (GB, CA, NL, ...) cannot be represented: it is **required (> 0) only for countries with purely numeric postcodes** — US, MX, DE, FR, IT, ES, AT, CH, BE, DK, NO, SE, FI, AU — and otherwise may be 0. A negative zip is always rejected. Its format/range is not checked.

A recovery interceptor turns any handler panic into `INTERNAL` instead of crashing the process (defence in depth; the handlers are written not to panic).

## Configuration

| Variable | Default | Description |
|---|---|---|
| `PORT` | `50051` | gRPC listen port |
| `METRICS_PORT` | `9090` | Port of the Prometheus `/metrics` endpoint (scraped by Alloy). A value that is not a port falls back to `9090` with a warning |
| `PPROF_ADDR` | unset (no pprof) | `host:port` of a separate pprof listener, e.g. `127.0.0.1:6060`. Off unless set; never expose it. pprof is never served on the metrics port |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`; anything else falls back to `info` with a warning |
| `SERVICE_VERSION` | build info, else `dev` | `service.version`, the `version` log field and the `build_info` label. Fallbacks in order: ldflags `-X <module>/internal/obs.Version=...` (and `.Commit=`, `.BuildTime=`), the VCS revision Go embeds in the binary (12 characters, `-dirty` if modified), `dev` |
| `DEPLOYMENT_ENVIRONMENT` | `local` | `deployment.environment` on every signal and the Pyroscope tag `environment` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | unset (telemetry off) | OTLP/**gRPC** endpoint for **traces, metrics and logs**, e.g. `http://alloy:4317`. Telemetry is on only if this is a valid absolute http(s) URL; empty or malformed means off (one warning, never a crash). The legacy `COLLECTOR_SERVICE_ADDR` and `ENABLE_TRACING` are no longer read |
| `OTEL_SDK_DISABLED` | unset | `true` (trimmed, case-insensitive) forces everything off: OTLP export and profiling, even if an endpoint or `PYROSCOPE_ADDR` is set |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` | Documented knob; only gRPC is implemented, the value is not read |
| `OTEL_TRACES_SAMPLER`, `OTEL_TRACES_SAMPLER_ARG` | SDK default (`parentbased_always_on`) | Standard OpenTelemetry sampler variables, read by the SDK (tested for `always_on` and `always_off`) |
| `OTEL_RESOURCE_ATTRIBUTES` | unset | Extra resource attributes (`k=v,...`); applied after the defaults, so they can override them |
| `OTEL_SERVICE_NAME` | `shipping-service` | `service.name`; keep the default, dashboards depend on it |
| `OTEL_METRIC_EXPORT_INTERVAL` | `15000` | Milliseconds between metric pushes (also accepts durations such as `15s`) |
| `PYROSCOPE_ADDR` | unset (profiling off) | Pyroscope URL (SDK push protocol, not OTLP). The only switch: no default address, `ENABLE_PROFILING=1` alone starts nothing |
| `ENABLE_PROFILING` | unset | `0` forces profiling off even with `PYROSCOPE_ADDR`; any other value changes nothing |

Notes:

- The Pyroscope application name follows `service.name` (`OTEL_SERVICE_NAME`, default `shipping-service`).
- Traces, metrics and logs share one OTLP/gRPC endpoint (Alloy); profiles go to Pyroscope. `service.instance.id` is the pod hostname, so replicas stay distinct in Mimir. Metrics are also scraped from `:9090/metrics`.
- `/metrics` on `METRICS_PORT` (default 9090) serves the Prometheus registry plus `build_info`; the same registry is bridged into OTLP, so metric names are the same on both paths. pprof used to be registered on this port unconditionally; it is now off unless `PPROF_ADDR` is set.
- Logs: JSON on stdout in the shape described under Observability, mirrored over OTLP when telemetry is on. The handlers log with `log.WithContext(ctx)` so the line carries `trace_id`/`span_id`. The address of an order is never logged (only the validation message, the ISO country and the zone).

## Container image

Multi-stage [`Dockerfile`](Dockerfile):

| Stage | Base | What happens |
|---|---|---|
| builder | `golang:1.25.9-alpine` | `go mod download`, then `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" ./cmd/server` |
| runtime | `alpine:3.21` | single binary at `/app/shipping-service` |

- **Entrypoint:** `/app/shipping-service`. `GOTRACEBACK=single`.
- **User: root** — no `USER` in the Dockerfile; the image runs as UID 0 (verified). Set a non-root `runAsUser` (e.g. `10001`) in Kubernetes; the binary needs no privileges.
- **Architecture: `amd64` only** (`GOARCH=amd64` hard-coded) — `exec format error` on `arm64` nodes.
- **EXPOSE:** `50051`, `9090`. **No `HEALTHCHECK`.**
- **Image size:** ~10 MB. No `.dockerignore`, so `.git` is sent in the build context.

## Kubernetes deployment notes

**Workload:** stateless `Deployment`; scale freely (callers pin connections per pod — see "gRPC load balancing" in the master README).

**Ports to declare**

| Name | Port | Protocol | Expose via Service? |
|---|---|---|---|
| `grpc` | `50051` | TCP (HTTP/2) | Yes — callers dial `shipping-service:50051` |
| `metrics` | `9090` | TCP (HTTP/1.1) | Prometheus `/metrics` (scraped by Alloy in the local stack; the same metrics are also pushed over OTLP). pprof is not served here |

**Service DNS name expected by callers:** `shipping-service` (`frontend` and `checkout-service`: `SHIPPING_SERVICE_ADDR=shipping-service:50051`). `payment-service` also uses `50051` — different Service, no clash.

**Probes**

```yaml
readinessProbe:
  grpc: { port: 50051 }
  periodSeconds: 5
livenessProbe:
  grpc: { port: 50051 }
  periodSeconds: 10
  failureThreshold: 3
```

Startup is near-instant; no `startupProbe` needed.

**Security context**

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 10001        # image runs as root by default; any non-zero UID works
  runAsGroup: 10001
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities: { drop: ["ALL"] }
```

**Resources** — requests sized from measured idle usage in the test stack (see the chart's `values.yaml` for limits):

```yaml
resources:
  requests: { cpu: 50m, memory: 32Mi }
  limits:   { cpu: "500m", memory: 128Mi }
```

**Config / secrets:** none.

**Telemetry:** pushed, not scraped — OTLP/gRPC to Alloy (`global.monitoring.otlpEndpoint`), profiles to Alloy `:4040`. Metrics carry `job="shipping-service"` and `instance=<pod>`; no scrape annotations or `ServiceMonitor` are needed.

**Helm chart:** [`platform-demo-helm/shipping-service`](https://github.com/platform-boutique/platform-demo-helm/tree/main/shipping-service) (built on the shared `platform-common` library chart). It ships with HA defaults — 2 replicas,
HPA, PodDisruptionBudget, topology spread, NetworkPolicy, restricted pod security context, probes and the full env contract (see Configuration) —
and was verified in the earlier local phase on a 3-node kind cluster (since deleted) under the `restricted` Pod Security Standard; it has not been verified on EKS yet.

**Deployment and admission policies:** Argo CD deploys the chart from `platform-demo-helm` (`envs/<env>/values/shipping-service.yaml` pins the image digest; promotion is a pull request). In the cluster Kyverno enforces, for this service: pods named `app.kubernetes.io/name: shipping-service` that run under the ServiceAccount of the same name (Cilium derives the network identity from that label), an image from this repository pinned by digest and signed by our release pipeline, a read-only root filesystem, resource requests and a memory limit, readiness and liveness probes and no API token. The Helm CI evaluates the rendered chart against these policies before merge; see [`platform-demo-helm/policies`](https://github.com/platform-boutique/platform-demo-helm/tree/main/policies).

## Observability

| Signal | Implementation | Destination |
|---|---|---|
| **Traces** | OpenTelemetry (`otelgrpc` handlers), OTLP/gRPC exporter (batch processor, queue bounded at 2048), W3C TraceContext + Baggage propagated to every downstream call. Sampler from `OTEL_TRACES_SAMPLER` (SDK default `parentbased_always_on`) | Alloy `:4317` -> Tempo |
| **Metrics** | Prometheus default registry: `grpc_server_handled_total`, `grpc_server_handling_seconds_*`, `grpc_server_started_total`, `grpc_server_msg_*`, Go runtime/process. Served as Prometheus text on `:9090/metrics` (`METRICS_PORT`) and, because the same registry is bridged into OpenTelemetry, also pushed over OTLP every 15 s (metric names identical). Plus `build_info{service,version,commit} 1` | Alloy scrapes `:9090` -> Mimir |
| **Logs** | `logrus`, one JSON object per line on stdout, plus an OTLP/gRPC hook (`otellogrus`) | stdout -> Alloy -> Loki; OTLP -> Alloy -> Loki |
| **Profiles** | Pyroscope Go SDK (CPU, alloc/in-use objects and space, goroutines, mutex, block), only with `PYROSCOPE_ADDR`; tags `service_name`, `version`, `environment` | Pyroscope |

**Telemetry contract** (the code is `internal/obs`, the same package in every Go service): telemetry is on only if `OTEL_EXPORTER_OTLP_ENDPOINT` is a valid absolute http(s) URL; `OTEL_SDK_DISABLED=true` forces everything off (profiling too); profiling only with `PYROSCOPE_ADDR`. Start never waits for the collector (the exporters connect in the background) and on SIGTERM the flush is cut off after 4 s, so a blackholed collector cannot hold the process (unit-tested with a listener that never answers: start and shutdown both return within the bound).

**Resource attributes** on every signal: `service.name` (`OTEL_SERVICE_NAME`, default `shipping-service`), `service.version` (`SERVICE_VERSION`, else build info, else `dev`), `service.namespace=everly`, `deployment.environment` (`DEPLOYMENT_ENVIRONMENT`, default `local`), `service.instance.id` (hostname). `OTEL_RESOURCE_ATTRIBUTES` is applied last.

**Log shape** (one shape for all services): `timestamp` (RFC 3339, UTC, milliseconds), `severity` (`debug`, `info`, `warning`, `error`), `message`, `service`, `version`, `trace_id` and `span_id` (hex, only while a valid span is active, never empty or all zeros), `request_id` (when the caller sent one), `error` (string, on errors), plus the call's own fields (for example `order_id`). Level from `LOG_LEVEL`. `trace_id`/`span_id` come from the context of the log call: handlers and clients log with `log.WithContext(ctx)`, the formatter reads the span from it. Calls without a context (startup, background work) have no ids. No card data, tokens or e-mail addresses are logged.

**Startup and version**: the first line is `message: "starting"` with `version`, `commit` and `environment`; `version`/`commit` come from `SERVICE_VERSION`, ldflags `-X <module>/internal/obs.Version=` / `.Commit=`, or the VCS data Go embeds (no Dockerfile change was made: without any of them a container image reports `version: "dev"`, `commit: "unknown"`). Panics are logged as one JSON line (`message: "panic"`, `error`) before the process dies; a startup error is one JSON error line, exit 1.

**pprof** is off. `PPROF_ADDR=127.0.0.1:6060` starts a separate listener; it is never on the metrics port.

**Graceful shutdown** (SIGTERM or SIGINT): the health status turns `NOT_SERVING`, the gRPC server stops accepting and lets in-flight calls finish (`GracefulStop`, forced stop after 5 s), then the telemetry flush runs, cut off after 4 s: the process exits within 9 s even when the collector is blackholed. Run against a listener that never answers (OTLP and Pyroscope both pointed at it) the binary exited 0 about 4 s after SIGTERM (smoke run, not part of the unit tests).

Every series carries `job="shipping-service"` (from `service.name`) and `instance=<pod name>` (`service.instance.id`), which is what the Grafana dashboards select on. Logs land in Loki with `service_name="shipping-service"` and trace/span ids, so a log line links to its trace.

## Running locally

Requirements: Go 1.25+, Docker, [`grpcurl`](https://github.com/fullstorydev/grpcurl).

```bash
go build ./... && go run ./cmd/server

docker build -t shipping-service .
docker run --rm -p 50051:50051 -p 9090:9090 shipping-service

go test ./...
```

### Manual gRPC testing (reflection is on)

```bash
grpcurl -plaintext -d '{
  "address": {"street_address": "1 Main St", "city": "Brooklyn", "state": "NY", "country": "US", "zip_code": 11201},
  "items": [{"product_id": "OLJCESPC7Z", "quantity": 2}]
}' localhost:50051 hipstershop.ShippingService/GetQuote

grpcurl -plaintext -d '{
  "address": {"street_address": "1 Main St", "city": "Brooklyn", "state": "NY", "country": "US", "zip_code": 11201},
  "items": [{"product_id": "OLJCESPC7Z", "quantity": 1}]
}' localhost:50051 hipstershop.ShippingService/ShipOrder

grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
curl -s localhost:9090/metrics | grep grpc_server
# pprof is off; PPROF_ADDR=127.0.0.1:6060 serves it on its own port
```

## Project structure

```
├── cmd/server/main.go            # entrypoint: obs.Start, gRPC server, graceful shutdown
├── internal/obs/                 # observability kit (env contract, JSON logs with trace ids, OTLP, build_info, :9090); same package in every Go service
├── internal/shipping/
│   ├── service.go                # GetQuote / ShipOrder handlers, recovery interceptor
│   ├── quote.go                  # zones, tariff table, quote logic
│   ├── address.go                # country normalisation, ShipOrder address validation
│   ├── countries.go              # ISO country list, aliases, ResolveCountry (shared by GetQuote and ShipOrder)
│   └── tracker.go                # deterministic tracking-ID generation
├── proto/                        # shipping.proto + generated Go
├── Dockerfile
├── go.mod / go.sum
├── .trivyignore
└── docs/                         # empty placeholder
```

## Known limitations

- Simulation only: nothing is actually shipped or stored; the tracking ID is a hash of the order content and cannot be looked up.
- Quotes ignore weight and size (the request has none) and the state/zip; the zone table is a fixed, made-up tariff.
- Address validation is lenient: it checks presence, the country and (for numeric-postcode countries) that a zip is given, not that the address exists or that the zip matches the city. `GetQuote` checks only the country (when given).
- `ShipOrder` is called by checkout after the card is charged. A country that `ShipOrder` would reject is now rejected by `GetQuote` first (checkout quotes before charging), but a street/city/zip problem is only found by `ShipOrder`, i.e. after payment, until checkout refunds on a failed shipment.
- pprof is off unless `PPROF_ADDR` is set (it is never on the metrics port).
- `amd64`-only image that runs as root unless the pod overrides it.
