# 1. The tracking id is a hash of the order content

Status: accepted (reconstructed from `internal/shipping/tracker.go` and the earlier README).

## Context

`ShipOrder` is called once per order by checkout, but a retry is possible (a lost answer, a replayed request). The service has no database, so it cannot remember which ids it handed out. Nothing is actually shipped.

## Decision

The tracking id is derived from the canonical text of the order (trimmed street, city, state, normalised ISO country code, zip, and the items as product id and summed quantity, sorted by product id) with SHA-256. The same order content always gives the same id, whatever the order of the items.

## Consequences

- A retried `ShipOrder` returns the same id and the service stays stateless.
- Two different orders with identical content (same address and items) get the same id.
- The id cannot be looked up anywhere; it is only a reference string.
