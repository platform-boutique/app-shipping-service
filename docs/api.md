# API

Package `hipstershop`, service `ShippingService` ([`proto/shipping.proto`](https://github.com/platform-boutique/app-shipping-service/blob/main/proto/shipping.proto)).

| Method | Request | Response | Behaviour |
|---|---|---|---|
| `GetQuote` | `{ Address address, repeated CartItem items }` | `{ Money cost_usd, int32 estimated_days, string zone }` | Prices by **destination zone and item count** (see the tariff below). Never rejects an address: a **nil address or blank country uses the default zone (`domestic`)**, an unrecognised country is priced as `rest_of_world`. 0 items → $0.00 and `estimated_days` 0. Always USD |
| `ShipOrder` | `{ Address address, repeated CartItem items }` | `{ string tracking_id }` | Validates the address (rules below); a nil or invalid address → `INVALID_ARGUMENT` with a message listing every problem. Otherwise builds a tracking ID like `XK-21345-109876543` from random letters/digits and the length of `"<street>, <city>, <state>"`. Nothing is dispatched or stored |

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

### Address validation (`ShipOrder` only)

Deliberately lenient:

- `street_address`, `city` and `country` must be non-blank; `state` is not checked.
- `country` must be an ISO 3166-1 alpha-2 code (any case, all 249 assigned codes) or one of about 100 known names/aliases (e.g. `United States`, `USA`, `U.S.A.`, `UK`, `Great Britain`, `Deutschland`), normalised to the ISO code. The frontend form sends `United States` by default.
- `zip_code` is an `int32`, so alphanumeric postcodes (GB, CA, NL, ...) cannot be represented: it is **required (> 0) only for countries with purely numeric postcodes** — US, MX, DE, FR, IT, ES, AT, CH, BE, DK, NO, SE, FI, AU — and otherwise may be 0. A negative zip is always rejected. Its format/range is not checked.

A recovery interceptor turns any handler panic into `INTERNAL` instead of crashing the process (defence in depth; the handlers are written not to panic).
