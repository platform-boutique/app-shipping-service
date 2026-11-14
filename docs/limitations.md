# Known limitations

- Simulation only: nothing is actually shipped or stored; the tracking ID is a hash of the order content and cannot be looked up.
- Quotes ignore weight and size (the request has none) and the state/zip; the zone table is a fixed, made-up tariff.
- Address validation is lenient: it checks presence, the country and (for numeric-postcode countries) that a zip is given, not that the address exists or that the zip matches the city. `GetQuote` checks only the country (when given).
- `ShipOrder` is called by checkout after the card is charged. A country that `ShipOrder` would reject is now rejected by `GetQuote` first (checkout quotes before charging), but a street/city/zip problem is only found by `ShipOrder`, i.e. after payment, until checkout refunds on a failed shipment.
- pprof is off unless `PPROF_ADDR` is set (it is never on the metrics port).
- `amd64`-only image that runs as root unless the pod overrides it.
