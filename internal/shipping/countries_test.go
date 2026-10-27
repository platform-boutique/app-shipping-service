package shipping

import (
	"context"
	"strings"
	"testing"

	pb "shipping-service/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// auditCountries are the ones the backend audit found accepted by GetQuote
// and rejected by ShipOrder (finding S1).
var auditCountries = map[string]string{
	"Thailand": "TH", "Taiwan": "TW", "Vietnam": "VN", "Indonesia": "ID", "Philippines": "PH",
	"Pakistan": "PK", "Kosovo": "XK", "Hong Kong": "HK", "Ivory Coast": "CI",
}

func TestAuditCountriesResolve(t *testing.T) {
	for name, want := range auditCountries {
		got, err := ResolveCountry(name)
		if err != nil || got != want {
			t.Errorf("ResolveCountry(%q) = %q,%v; want %q", name, got, err, want)
		}
	}
}

// Every ISO code has a name in countryList and vice versa (plus XK).
func TestCountryListMatchesISOCodes(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range countryList {
		if seen[c.Code] {
			t.Errorf("duplicate code %s in countryList", c.Code)
		}
		seen[c.Code] = true
		if !isoCodes[c.Code] && !extraCodes[c.Code] {
			t.Errorf("%s (%s) is neither ISO nor an extra code", c.Code, c.Name)
		}
	}
	for code := range isoCodes {
		if !seen[code] {
			t.Errorf("ISO code %s has no name in countryList", code)
		}
	}
	if len(seen) != len(isoCodes)+len(extraCodes) {
		t.Errorf("countryList has %d entries; want %d", len(seen), len(isoCodes)+len(extraCodes))
	}
}

// The core of S1: for every accepted country (each name, each alias, each
// code) GetQuote and ShipOrder agree, so a quote that succeeds is never
// followed by a rejected shipment.
func TestQuoteAndShipAgreeForEveryCountry(t *testing.T) {
	svc := newTestService()
	inputs := map[string]string{} // input -> expected code
	for _, c := range countryList {
		inputs[c.Name] = c.Code
		inputs[c.Code] = c.Code
		inputs[strings.ToLower(c.Code)] = c.Code
		inputs[strings.ToUpper(c.Name)] = c.Code
	}
	for k, v := range countryNames {
		inputs[k] = v
	}
	for k, v := range countryAliases {
		inputs[k] = v
	}
	for in, want := range inputs {
		addr := &pb.Address{StreetAddress: "1 Main St", City: "Town", Country: in, ZipCode: 12345}
		q, qerr := svc.GetQuote(context.Background(), &pb.GetQuoteRequest{Address: addr, Items: items(1)})
		_, serr := svc.ShipOrder(context.Background(), &pb.ShipOrderRequest{Address: addr, Items: items(1)})
		if qerr != nil || serr != nil {
			t.Errorf("%q: quote err=%v ship err=%v; both must accept", in, qerr, serr)
			continue
		}
		if got, _ := ResolveCountry(in); got != want {
			t.Errorf("%q resolved to %q; want %q", in, got, want)
		}
		if q.GetZone() != string(ZoneForCountry(want)) {
			t.Errorf("%q: zone %s; want %s", in, q.GetZone(), ZoneForCountry(want))
		}
	}
}

// Regression S1: an unsupported country fails at quote time with a readable
// INVALID_ARGUMENT, and ShipOrder agrees.
func TestUnsupportedCountryFailsAtQuoteTime(t *testing.T) {
	svc := newTestService()
	for _, bad := range []string{"Atlantis", "Narnia", "ZZ", "U", "12345", "Thailnd"} {
		addr := &pb.Address{StreetAddress: "1 Main St", City: "Town", Country: bad, ZipCode: 12345}
		_, qerr := svc.GetQuote(context.Background(), &pb.GetQuoteRequest{Address: addr, Items: items(1)})
		if status.Code(qerr) != codes.InvalidArgument {
			t.Fatalf("GetQuote(%q): want InvalidArgument, got %v", bad, qerr)
		}
		for _, want := range []string{"invalid address", "country", bad, "ISO 3166-1"} {
			if !strings.Contains(qerr.Error(), want) {
				t.Errorf("GetQuote(%q) message %q should contain %q", bad, qerr.Error(), want)
			}
		}
		if _, serr := svc.ShipOrder(context.Background(), &pb.ShipOrderRequest{Address: addr, Items: items(1)}); status.Code(serr) != codes.InvalidArgument {
			t.Errorf("ShipOrder(%q): want InvalidArgument, got %v", bad, serr)
		}
	}
}

// The cart-page estimate sends no country (or a blank one): still priced.
func TestGetQuote_BlankOrMissingCountryStillWorks(t *testing.T) {
	svc := newTestService()
	for _, a := range []*pb.Address{nil, {}, {Country: "   "}} {
		if _, err := svc.GetQuote(context.Background(), &pb.GetQuoteRequest{Address: a, Items: items(1)}); err != nil {
			t.Errorf("address %v: unexpected error %v", a, err)
		}
	}
}

func TestResolveCountry_Variants(t *testing.T) {
	cases := map[string]string{
		"côte d'ivoire": "CI", "Cote d’Ivoire": "CI", "COTE DIVOIRE": "CI", "  hong   kong ": "HK",
		"Türkiye": "TR", "turkey": "TR", "São Tomé and Príncipe": "ST", "Guinea-Bissau": "GW",
		"Bosnia & Herzegovina": "BA", "xk": "XK", "Korea, South": "KR", "Czech Republic": "CZ",
	}
	for in, want := range cases {
		if got, err := ResolveCountry(in); err != nil || got != want {
			t.Errorf("ResolveCountry(%q) = %q,%v; want %q", in, got, err, want)
		}
	}
}

func TestResolveCountry_LongInputIsTruncatedInMessage(t *testing.T) {
	_, err := ResolveCountry(strings.Repeat("x", 5000))
	if err == nil || len(err.Error()) > 300 {
		t.Fatalf("want a short error, got %v", err)
	}
}

func TestResolveCountry_Blank(t *testing.T) {
	if _, err := ResolveCountry("  "); err != ErrCountryRequired {
		t.Errorf("got %v", err)
	}
}
