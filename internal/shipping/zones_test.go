package shipping

import (
	"context"
	"io"
	"strings"
	"testing"

	pb "shipping-service/proto"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newTestService() *Service {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return NewService(l)
}

func items(qty ...int32) []*pb.CartItem {
	out := make([]*pb.CartItem, 0, len(qty))
	for _, q := range qty {
		out = append(out, &pb.CartItem{ProductId: "P", Quantity: q})
	}
	return out
}

func validUS() *pb.Address {
	return &pb.Address{StreetAddress: "1600 Amphitheatre Pkwy", City: "Mountain View", State: "CA", Country: "United States", ZipCode: 94043}
}

func cents(m *pb.Money) int64 { return m.GetUnits()*100 + int64(m.GetNanos()/10_000_000) }

// ---------------------------------------------------------------------------
// Countries and zones
// ---------------------------------------------------------------------------

func TestISOListComplete(t *testing.T) {
	if n := len(isoCodes); n != 249 {
		t.Errorf("expected 249 ISO 3166-1 alpha-2 codes, got %d", n)
	}
}

func TestNormalizeCountry(t *testing.T) {
	cases := map[string]string{
		"US": "US", "us": "US", " United States ": "US", "U.S.A.": "US", "united  states of america": "US",
		"UK": "GB", "gb": "GB", "Great Britain": "GB", "Deutschland": "DE", "de": "DE",
		"Canada": "CA", "mx": "MX", "Japan": "JP", "nz": "NZ",
	}
	for in, want := range cases {
		got, ok := NormalizeCountry(in)
		if !ok || got != want {
			t.Errorf("NormalizeCountry(%q) = %q,%v; want %q,true", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "  ", "XX", "Atlantis", "U", "USAA", "ZZ"} {
		if got, ok := NormalizeCountry(in); ok {
			t.Errorf("NormalizeCountry(%q) = %q,true; want rejection", in, got)
		}
	}
}

func TestZoneForCountry(t *testing.T) {
	cases := map[string]Zone{
		"US": ZoneDomestic,
		"CA": ZoneNorthAmerica, "MX": ZoneNorthAmerica,
		"DE": ZoneEurope, "FR": ZoneEurope, "GB": ZoneEurope, "CH": ZoneEurope, "NO": ZoneEurope, "MK": ZoneEurope,
		"JP": ZoneRestOfWorld, "AU": ZoneRestOfWorld, "BR": ZoneRestOfWorld, "RU": ZoneRestOfWorld, "PR": ZoneRestOfWorld,
	}
	for code, want := range cases {
		if got := ZoneForCountry(code); got != want {
			t.Errorf("ZoneForCountry(%s) = %s; want %s", code, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// QuoteFor
// ---------------------------------------------------------------------------

func TestQuoteFor_ZeroItemsIsFree(t *testing.T) {
	for z := range Rates {
		if q := QuoteFor(z, 0); q != (Quote{}) {
			t.Errorf("zone %s, 0 items: got %v, want $0.00", z, q)
		}
		if q := QuoteFor(z, -3); q != (Quote{}) {
			t.Errorf("zone %s, negative count: got %v, want $0.00", z, q)
		}
	}
}

func TestQuoteFor_EveryZone(t *testing.T) {
	cases := []struct {
		zone    Zone
		count   int
		dollars uint32
		cents   uint32
	}{
		{ZoneDomestic, 1, 8, 99}, // unchanged from the former flat rate
		{ZoneDomestic, 3, 11, 99},
		{ZoneNorthAmerica, 1, 14, 99},
		{ZoneNorthAmerica, 2, 17, 49},
		{ZoneEurope, 1, 19, 99},
		{ZoneEurope, 4, 28, 99},
		{ZoneRestOfWorld, 1, 29, 99},
		{ZoneRestOfWorld, 2, 34, 49},
		{Zone("unknown"), 1, 29, 99}, // falls back to the rest-of-world tariff
	}
	for _, c := range cases {
		q := QuoteFor(c.zone, c.count)
		if q.Dollars != c.dollars || q.Cents != c.cents {
			t.Errorf("QuoteFor(%s,%d) = $%d.%02d; want $%d.%02d", c.zone, c.count, q.Dollars, q.Cents, c.dollars, c.cents)
		}
	}
}

func TestQuoteFor_HugeCountDoesNotOverflow(t *testing.T) {
	q := QuoteFor(ZoneRestOfWorld, 1<<40)
	if q.Dollars == 0 {
		t.Errorf("expected a large capped quote, got %v", q)
	}
}

// ---------------------------------------------------------------------------
// GetQuote
// ---------------------------------------------------------------------------

func TestGetQuote_NilAddressUsesDefaultZone(t *testing.T) {
	resp, err := newTestService().GetQuote(context.Background(), &pb.GetQuoteRequest{Items: items(1)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetZone() != string(DefaultZone) || cents(resp.GetCostUsd()) != 899 || resp.GetCostUsd().GetCurrencyCode() != "USD" {
		t.Errorf("got zone=%q cost=%v", resp.GetZone(), resp.GetCostUsd())
	}
	if resp.GetEstimatedDays() != Rates[DefaultZone].EstimatedDays {
		t.Errorf("estimated_days = %d", resp.GetEstimatedDays())
	}
}

func TestGetQuote_EveryZoneViaRPC(t *testing.T) {
	cases := []struct {
		country string
		zone    Zone
		cents   int64
		days    int32
	}{
		{"United States", ZoneDomestic, 899 + 150, 3},
		{"Canada", ZoneNorthAmerica, 1499 + 250, 6},
		{"MX", ZoneNorthAmerica, 1499 + 250, 6},
		{"Germany", ZoneEurope, 1999 + 300, 8},
		{"uk", ZoneEurope, 1999 + 300, 8},
		{"Japan", ZoneRestOfWorld, 2999 + 450, 14},
		{"Thailand", ZoneRestOfWorld, 2999 + 450, 14},
		{"", DefaultZone, 899 + 150, 3}, // blank: default zone
	}
	svc := newTestService()
	for _, c := range cases {
		resp, err := svc.GetQuote(context.Background(), &pb.GetQuoteRequest{
			Address: &pb.Address{Country: c.country},
			Items:   items(1, 1),
		})
		if err != nil {
			t.Fatalf("%q: unexpected error: %v", c.country, err)
		}
		if resp.GetZone() != string(c.zone) || cents(resp.GetCostUsd()) != c.cents || resp.GetEstimatedDays() != c.days {
			t.Errorf("%q: got zone=%s cents=%d days=%d; want %s %d %d",
				c.country, resp.GetZone(), cents(resp.GetCostUsd()), resp.GetEstimatedDays(), c.zone, c.cents, c.days)
		}
	}
}

func TestGetQuote_ZeroItems(t *testing.T) {
	resp, err := newTestService().GetQuote(context.Background(), &pb.GetQuoteRequest{Address: validUS(), Items: items(0, -2)})
	if err != nil {
		t.Fatal(err)
	}
	if cents(resp.GetCostUsd()) != 0 || resp.GetCostUsd().GetCurrencyCode() != "USD" || resp.GetEstimatedDays() != 0 {
		t.Errorf("expected $0.00 USD and 0 days, got %v days=%d", resp.GetCostUsd(), resp.GetEstimatedDays())
	}
}

func TestGetQuote_NilRequestFields(t *testing.T) {
	if _, err := newTestService().GetQuote(context.Background(), &pb.GetQuoteRequest{}); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// ShipOrder / ValidateAddress
// ---------------------------------------------------------------------------

// Regression: a nil address used to dereference a nil pointer and crash the process.
func TestShipOrder_NilAddressNoLongerPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ShipOrder panicked: %v", r)
		}
	}()
	_, err := newTestService().ShipOrder(context.Background(), &pb.ShipOrderRequest{Items: items(1)})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if !strings.Contains(err.Error(), "address is required") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestShipOrder_EmptyAddress(t *testing.T) {
	_, err := newTestService().ShipOrder(context.Background(), &pb.ShipOrderRequest{Address: &pb.Address{}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	for _, want := range []string{"street_address", "city", "country"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q should mention %s", err.Error(), want)
		}
	}
}

func TestShipOrder_ValidAddress(t *testing.T) {
	resp, err := newTestService().ShipOrder(context.Background(), &pb.ShipOrderRequest{Address: validUS(), Items: items(1)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !trackingPattern.MatchString(resp.GetTrackingId()) {
		t.Errorf("bad tracking id %q", resp.GetTrackingId())
	}
}

func TestValidateAddress(t *testing.T) {
	mod := func(f func(a *pb.Address)) *pb.Address { a := validUS(); f(a); return a }
	valid := []struct {
		name string
		addr *pb.Address
		code string
	}{
		{"US by name", validUS(), "US"},
		{"US by code lower case", mod(func(a *pb.Address) { a.Country = "us" }), "US"},
		{"GB without zip (alphanumeric postcodes)", &pb.Address{StreetAddress: "10 Downing St", City: "London", Country: "United Kingdom"}, "GB"},
		{"CA without zip", &pb.Address{StreetAddress: "1 Yonge St", City: "Toronto", Country: "CA"}, "CA"},
		{"DE with zip", &pb.Address{StreetAddress: "Unter den Linden 1", City: "Berlin", Country: "Germany", ZipCode: 10117}, "DE"},
		{"state not required", mod(func(a *pb.Address) { a.State = "" }), "US"},
	}
	for _, c := range valid {
		code, err := ValidateAddress(c.addr)
		if err != nil || code != c.code {
			t.Errorf("%s: got %q,%v; want %q,nil", c.name, code, err, c.code)
		}
	}

	invalid := []struct {
		name string
		addr *pb.Address
		want string
	}{
		{"nil", nil, "address is required"},
		{"blank street", mod(func(a *pb.Address) { a.StreetAddress = "   " }), "street_address"},
		{"blank city", mod(func(a *pb.Address) { a.City = "" }), "city"},
		{"blank country", mod(func(a *pb.Address) { a.Country = "" }), "country is required"},
		{"unknown country", mod(func(a *pb.Address) { a.Country = "Atlantis" }), "not an ISO"},
		{"US zero zip", mod(func(a *pb.Address) { a.ZipCode = 0 }), "zip_code is required for country US"},
		{"DE zero zip", &pb.Address{StreetAddress: "x", City: "Berlin", Country: "DE"}, "zip_code is required for country DE"},
		{"negative zip", &pb.Address{StreetAddress: "x", City: "London", Country: "GB", ZipCode: -1}, "must not be negative"},
	}
	for _, c := range invalid {
		_, err := ValidateAddress(c.addr)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v; want error containing %q", c.name, err, c.want)
		}
	}
}

func TestShipOrder_InvalidAddressesAreInvalidArgument(t *testing.T) {
	svc := newTestService()
	for _, a := range []*pb.Address{
		{City: "x", Country: "US", ZipCode: 1},
		{StreetAddress: "x", Country: "US", ZipCode: 1},
		{StreetAddress: "x", City: "x", Country: "Nowhere"},
		{StreetAddress: "x", City: "x", Country: "US"},
	} {
		if _, err := svc.ShipOrder(context.Background(), &pb.ShipOrderRequest{Address: a}); status.Code(err) != codes.InvalidArgument {
			t.Errorf("address %v: expected InvalidArgument, got %v", a, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Recovery interceptor
// ---------------------------------------------------------------------------

func TestRecoveryInterceptor(t *testing.T) {
	l := logrus.New()
	l.SetOutput(io.Discard)
	icpt := RecoveryUnaryInterceptor(l)
	info := &grpc.UnaryServerInfo{FullMethod: "/hipstershop.ShippingService/ShipOrder"}

	_, err := icpt(context.Background(), nil, info, func(context.Context, any) (any, error) {
		var a *pb.Address
		_ = a.StreetAddress // nil dereference
		return nil, nil
	})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal after a panic, got %v", err)
	}

	resp, err := icpt(context.Background(), nil, info, func(context.Context, any) (any, error) { return "ok", nil })
	if err != nil || resp != "ok" {
		t.Errorf("pass-through broken: %v %v", resp, err)
	}
}
