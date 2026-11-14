# shipping-service

<div class="pd-hero" markdown>
<span class="pd-kicker">service · platform-demo</span>

A gRPC service that provides shipping quotes and "ships" orders for the platform-demo e-commerce platform. It is a simulation: quotes come from a made-up tariff by destination zone and item count, and `ShipOrder` validates the address and fabricates a tracking ID. It is stateless, has no downstream dependencies, and is called by `checkout-service` (and by `frontend` for the cart-page shipping estimate).
</div>

<div class="pd-facts" markdown>
<div><span>Runtime</span><strong>Go 1.25</strong><small>Go</small></div>
<div><span>Listens on</span><strong>50051</strong><small>gRPC</small></div>
<div><span>State</span><strong>Stateless</strong></div>
<div><span>Image</span><strong>shipping-service</strong><small>mladenovskistefan/</small></div>
</div>

## Where it sits

Called by `frontend`, `checkout-service`. It has no downstream dependencies.

```mermaid
flowchart LR
  shipping_service["<b>shipping-service</b><br/>50051 (gRPC)"]:::me
  frontend["frontend"] --> shipping_service
  checkout_service["checkout-service"] --> shipping_service
  classDef me stroke-width:3px;
```

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

## Find it elsewhere

<div class="pd-cards" markdown>
<div><h4><a href="https://argocd.platform-demo.click/applications/argocd/dev-shipping-service">Argo CD</a></h4><p>Sync and health of <code>dev-shipping-service</code>.</p></div>
<div><h4><a href="https://github.com/platform-boutique/app-shipping-service">Repository</a></h4><p>Source, pull requests and releases of <code>app-shipping-service</code>.</p></div>
</div>

!!! info "Part of the platform-demo platform"
    Cross-cutting facts (service wiring, observability contract, Kubernetes translation, CI/CD) are in the
    [master reference](https://github.com/platform-boutique/platform-demo-docs). This site only describes shipping-service.
