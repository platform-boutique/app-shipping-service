package shipping

import (
	"context"
	"testing"

	pb "shipping-service/proto"
)

func order(addr *pb.Address, its ...*pb.CartItem) *pb.ShipOrderRequest {
	return &pb.ShipOrderRequest{Address: addr, Items: its}
}

func item(id string, q int32) *pb.CartItem { return &pb.CartItem{ProductId: id, Quantity: q} }

// Regression S2: a retried ShipOrder for the same order returns the same id.
func TestShipOrder_RetryReturnsSameTrackingID(t *testing.T) {
	svc := newTestService()
	req := order(validUS(), item("a", 1), item("b", 2))
	first, err := svc.ShipOrder(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		again, err := svc.ShipOrder(context.Background(), req)
		if err != nil || again.GetTrackingId() != first.GetTrackingId() {
			t.Fatalf("retry %d: got %q,%v; want %q", i, again.GetTrackingId(), err, first.GetTrackingId())
		}
	}
	if !trackingPattern.MatchString(first.GetTrackingId()) {
		t.Errorf("bad format %q", first.GetTrackingId())
	}
}

func TestShipOrder_TrackingIDIgnoresItemOrderAndCountrySpelling(t *testing.T) {
	svc := newTestService()
	a, b := validUS(), validUS()
	b.Country = "us"
	r1, _ := svc.ShipOrder(context.Background(), order(a, item("a", 1), item("b", 2)))
	r2, _ := svc.ShipOrder(context.Background(), order(b, item("b", 1), item("a", 1), item("b", 1), item("z", 0)))
	if r1.GetTrackingId() != r2.GetTrackingId() {
		t.Errorf("same content must give the same id: %q vs %q", r1.GetTrackingId(), r2.GetTrackingId())
	}
}

func TestShipOrder_TrackingIDDiffersPerContent(t *testing.T) {
	svc := newTestService()
	base, _ := svc.ShipOrder(context.Background(), order(validUS(), item("a", 1)))
	otherItems, _ := svc.ShipOrder(context.Background(), order(validUS(), item("a", 2)))
	otherAddr := validUS()
	otherAddr.StreetAddress = "2 Other Rd"
	otherAddress, _ := svc.ShipOrder(context.Background(), order(otherAddr, item("a", 1)))
	if base.GetTrackingId() == otherItems.GetTrackingId() || base.GetTrackingId() == otherAddress.GetTrackingId() {
		t.Errorf("different content should give different ids: %q %q %q",
			base.GetTrackingId(), otherItems.GetTrackingId(), otherAddress.GetTrackingId())
	}
}

func TestTrackingID_Deterministic(t *testing.T) {
	first, second := TrackingID("x"), TrackingID("x")
	if first != second {
		t.Error("TrackingID must be deterministic")
	}
}

// Letters cover A-Z (the old random generator never produced Z).
func TestTrackingID_LettersIncludeZ(t *testing.T) {
	seen := map[byte]bool{}
	for i := 0; i < 20000; i++ {
		id := TrackingID(string(rune('a'+i%26)) + string(rune(i)))
		seen[id[0]] = true
		seen[id[1]] = true
	}
	if !seen['Z'] || !seen['A'] || len(seen) != 26 {
		t.Errorf("expected all 26 letters, got %d", len(seen))
	}
}
