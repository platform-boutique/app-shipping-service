# Known limitations

- Simulation only: nothing is actually shipped or stored; the tracking ID is random and cannot be looked up.
- Quotes ignore weight and size (the request has none) and the state/zip; the zone table is a fixed, made-up tariff.
- Address validation is lenient: it checks presence, the country and (for numeric-postcode countries) that a zip is given, not that the address exists or that the zip matches the city. `GetQuote` validates nothing.
- `ShipOrder` is called by checkout after the card is charged, so an address rejected here currently fails the order after payment (no refund until checkout's saga lands).
- pprof is always exposed on the metrics port.
- `amd64`-only image that runs as root unless the pod overrides it.
