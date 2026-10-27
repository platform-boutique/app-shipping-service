# Develop & test

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
curl -s localhost:9090/debug/pprof/ | head
```

## Project structure

```
├── cmd/server/main.go            # entrypoint: tracing, profiling, gRPC server, metrics + pprof server
├── internal/shipping/
│   ├── service.go                # GetQuote / ShipOrder handlers, recovery interceptor
│   ├── quote.go                  # zones, tariff table, quote logic
│   ├── address.go                # country normalisation, ShipOrder address validation
│   └── tracker.go                # random tracking-ID generation
├── proto/                        # shipping.proto + generated Go
├── Dockerfile
├── go.mod / go.sum
├── .trivyignore
└── docs/                         # empty placeholder
```
