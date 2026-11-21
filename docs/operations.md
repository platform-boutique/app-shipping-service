# Operations

## Health

- `grpc.health.v1.Health/Check` on the gRPC port (`50051`): `SERVING` once the server listens, `NOT_SERVING` for every service while the server drains after SIGTERM. It does not look at anything else because there is nothing else to look at: the service has no dependencies.
- Container probe: `/app/shipping-service healthcheck` (exec form, no shell) calls the local health service and exits 0 only on `SERVING`.

```bash
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
curl -s localhost:9090/metrics | grep grpc_server
```

## Shutdown

On SIGTERM or SIGINT the health service goes `NOT_SERVING`, the server stops accepting and `GracefulStop` lets in-flight calls finish for at most 5 s (`drainTimeout`), after which the server is cut off with `Stop`. The telemetry flush is bounded at 4 s (`obs.DefaultShutdownTimeout`), so the process exits within 9 s even with an unreachable collector. The drain path has unit tests (`cmd/server/main_test.go`); a measured `docker stop` was not run for this page.

## Scaling and replicas

Stateless: scale freely. The tracking id is derived from the order content, so every replica gives the same answer for the same order. Callers pin each gRPC connection to one pod.

## Failure behaviour

| Failure | Behaviour |
|---|---|
| Invalid address or unknown country in `ShipOrder` | `INVALID_ARGUMENT` with a message that lists every problem |
| Unknown country in `GetQuote` | `INVALID_ARGUMENT`, so checkout fails before the charge |
| Handler panic | turned into `INTERNAL` by a recovery interceptor, the process keeps running |
| Collector unreachable | telemetry export fails in the background; start-up does not wait for it |

## Runbook

| Symptom | Check |
|---|---|
| Checkout fails with `shipping: invalid address: ...` | the message names the field; for a country, use an ISO 3166-1 alpha-2 code or an English country name |
| Zip rejected | a zip is required (> 0) only for the countries with purely numeric postcodes listed in the README |
| Same tracking id twice | by design: the id is a hash of the order content, a retried `ShipOrder` returns the same id |
| Quote looks wrong | the zone of the country and the item count; see the tariff in [API](api.md) |

There is no dashboard or alert rule for this service in the repository yet (the dashboards are an open task).
