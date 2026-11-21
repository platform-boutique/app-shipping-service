# Develop & test

## Checks and commands

Requirements: Go 1.26 (`GOTOOLCHAIN=auto` fetches the `go.mod` version; the host has Go 1.22), Docker for the image, Python 3.12 for the docs check. The commands below are the ones the shared pipeline (`platform-demo-workflows`) runs, so a failure can be reproduced without pushing.

| Step | Command | Result on 2026-11-21 |
|---|---|---|
| Format | `gofmt -l .` | run, no files listed |
| Vet and unit tests | `go vet ./... && go test -race -count=1 ./...` | run, passed (4 packages with tests) |
| Lint | `golangci-lint run ./...` (CI pins golangci-lint v2.14.0) | not run (golangci-lint is not installed on the host) |
| Vulnerabilities and security | `govulncheck ./...`, `gosec -exclude-generated ./...` | not run |
| Proto | `protoc -I proto --go_out=proto --go_opt=paths=source_relative --go-grpc_out=proto --go-grpc_opt=paths=source_relative proto/shipping.proto` (protoc 3.21.12, protoc-gen-go 1.36.11, protoc-gen-go-grpc 1.6.1, the versions in the generated files) | not run. Changes are additive only, server first |
| Image | `docker build -t platform-demo/shipping-service:local .` | not run today |
| Image scan | `trivy image --severity CRITICAL,HIGH --exit-code 1 platform-demo/shipping-service:local` | not run today |
| Secret scan | `pre-commit run --all-files` (`.pre-commit-config.yaml`, gitleaks) or `gitleaks git --redact` | run with the gitleaks binary of the pre-commit cache (`gitleaks git --redact`, full history, 50 commits): no leaks found |
| Workflow lint | `actionlint` | run (actionlint 1.7.12), no findings |
| Docs | `mkdocs build --strict` (mkdocs 1.6.1 + mkdocs-techdocs-core 1.7.1, Python 3.12 venv) and the link check `python3 -I tools/check-md-links.py .` from `platform-demo-workflows` (relative links in `README.md` and `docs/`) | run, both passed |

## Running locally

Requirements: Go 1.26 (`GOTOOLCHAIN=auto` fetches the `go.mod` version), Docker, [`grpcurl`](https://github.com/fullstorydev/grpcurl).

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
├── internal/healthcheck/healthcheck.go     # `healthcheck` subcommand (container HEALTHCHECK): grpc.health.v1 Check on the own port
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
└── docs/                         # TechDocs pages (mkdocs.yml in the root), docs/adr/ = decisions
```
