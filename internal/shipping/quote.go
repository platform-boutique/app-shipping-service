package shipping

import (
	"fmt"
	"math"
)

// Quote represents a currency value.
type Quote struct {
	Dollars uint32
	Cents   uint32
}

// String representation of the Quote.
func (q Quote) String() string {
	return fmt.Sprintf("$%d.%d", q.Dollars, q.Cents)
}

// Zone is a destination shipping zone.
type Zone string

// Shipping zones. The string values are what GetQuoteResponse.zone carries.
const (
	ZoneDomestic     Zone = "domestic"      // US
	ZoneNorthAmerica Zone = "north_america" // CA, MX
	ZoneEurope       Zone = "europe"
	ZoneRestOfWorld  Zone = "rest_of_world"

	// DefaultZone is used by GetQuote when the request has no address or an
	// empty country (the frontend's cart-page estimate sends no address).
	DefaultZone = ZoneDomestic
)

// Rate is the simulated tariff of a zone, in US cents.
type Rate struct {
	BaseCents     uint32 // price of the first item
	PerExtraCents uint32 // price of every further item
	EstimatedDays int32  // simulated transit time
}

// Rates is the tariff table. Domestic with one item stays at the former flat
// $8.99, so the cart estimate for a single item does not change.
var Rates = map[Zone]Rate{
	ZoneDomestic:     {BaseCents: 899, PerExtraCents: 150, EstimatedDays: 3},
	ZoneNorthAmerica: {BaseCents: 1499, PerExtraCents: 250, EstimatedDays: 6},
	ZoneEurope:       {BaseCents: 1999, PerExtraCents: 300, EstimatedDays: 8},
	ZoneRestOfWorld:  {BaseCents: 2999, PerExtraCents: 450, EstimatedDays: 14},
}

var europe = map[string]bool{}

func init() {
	// EU + EEA + UK, Switzerland, the microstates and the non-EU Balkans/East.
	for _, c := range []string{
		"AT", "BE", "BG", "HR", "CY", "CZ", "DK", "EE", "FI", "FR", "DE", "GR", "HU", "IE", "IT",
		"LV", "LT", "LU", "MT", "NL", "PL", "PT", "RO", "SK", "SI", "ES", "SE",
		"IS", "LI", "NO", "CH", "GB", "AD", "MC", "SM", "VA",
		"AL", "BA", "ME", "MK", "RS", "MD", "UA", "BY",
		"AX", "FO", "GG", "GI", "IM", "JE", "SJ",
	} {
		europe[c] = true
	}
}

// ZoneForCountry maps a normalised ISO 3166-1 alpha-2 code to its zone.
func ZoneForCountry(code string) Zone {
	switch {
	case code == "US":
		return ZoneDomestic
	case code == "CA" || code == "MX":
		return ZoneNorthAmerica
	case europe[code]:
		return ZoneEurope
	default:
		return ZoneRestOfWorld
	}
}

// QuoteFor prices a shipment of count items to zone: 0 items costs $0.00,
// otherwise base + (count-1) * per-extra-item. The result is capped at
// math.MaxUint32 dollars, which no real cart reaches.
func QuoteFor(zone Zone, count int) Quote {
	if count <= 0 {
		return Quote{}
	}
	r, ok := Rates[zone]
	if !ok {
		r = Rates[ZoneRestOfWorld]
	}
	cents := uint64(r.BaseCents) + uint64(count-1)*uint64(r.PerExtraCents) //#nosec G115 // count > 0 here
	dollars := cents / 100
	if dollars > math.MaxUint32 {
		return Quote{Dollars: math.MaxUint32, Cents: 99}
	}
	return Quote{Dollars: uint32(dollars), Cents: uint32(cents % 100)} //#nosec G115 // bounded above
}

// QuoteFromFloat takes a price as a float and creates a Quote.
func QuoteFromFloat(value float64) Quote {
	units, fraction := math.Modf(value)
	return Quote{
		Dollars: uint32(units),
		Cents:   uint32(math.Trunc(fraction * 100)),
	}
}
