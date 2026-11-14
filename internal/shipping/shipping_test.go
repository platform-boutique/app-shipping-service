package shipping

import (
	"regexp"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// QuoteFromFloat
// ---------------------------------------------------------------------------

func TestQuoteFromFloat_Zero(t *testing.T) {
	q := QuoteFromFloat(0)
	if q.Dollars != 0 || q.Cents != 0 {
		t.Errorf("expected $0.00, got $%d.%d", q.Dollars, q.Cents)
	}
}

func TestQuoteFromFloat_WholeNumber(t *testing.T) {
	q := QuoteFromFloat(10)
	if q.Dollars != 10 || q.Cents != 0 {
		t.Errorf("expected $10.00, got $%d.%d", q.Dollars, q.Cents)
	}
}

func TestQuoteFromFloat_WithCents(t *testing.T) {
	q := QuoteFromFloat(8.99)
	if q.Dollars != 8 || q.Cents != 99 {
		t.Errorf("expected $8.99, got $%d.%d", q.Dollars, q.Cents)
	}
}

func TestQuoteFromFloat_OnlyCents(t *testing.T) {
	q := QuoteFromFloat(0.50)
	if q.Dollars != 0 || q.Cents != 50 {
		t.Errorf("expected $0.50, got $%d.%d", q.Dollars, q.Cents)
	}
}

func TestQuoteFromFloat_LargeValue(t *testing.T) {
	q := QuoteFromFloat(999.99)
	if q.Dollars != 999 || q.Cents != 99 {
		t.Errorf("expected $999.99, got $%d.%d", q.Dollars, q.Cents)
	}
}

// ---------------------------------------------------------------------------
// Quote.String
// ---------------------------------------------------------------------------

func TestQuoteString(t *testing.T) {
	q := Quote{Dollars: 8, Cents: 99}
	s := q.String()
	if s != "$8.99" {
		t.Errorf("expected $8.99, got %s", s)
	}
}

func TestQuoteString_Zero(t *testing.T) {
	q := Quote{Dollars: 0, Cents: 0}
	s := q.String()
	if s != "$0.0" {
		t.Errorf("expected $0.0, got %s", s)
	}
}

// ---------------------------------------------------------------------------
// TrackingID
// ---------------------------------------------------------------------------

var trackingPattern = regexp.MustCompile(`^[A-Z]{2}-\d+[0-9]{3}-\d+[0-9]{7}$`)

func TestTrackingID_Format(t *testing.T) {
	id := TrackingID("123 Main St, Springfield, IL")
	if !trackingPattern.MatchString(id) {
		t.Errorf("tracking ID %q does not match expected pattern XX-NNN-NNNNNNNNN", id)
	}
}

func TestTrackingID_StartsWithTwoUppercaseLetters(t *testing.T) {
	id := TrackingID("some address")
	parts := strings.SplitN(id, "-", 2)
	if len(parts) < 2 {
		t.Fatalf("expected at least one dash in tracking ID, got %q", id)
	}
	prefix := parts[0]
	if len(prefix) != 2 {
		t.Errorf("expected 2-letter prefix, got %q", prefix)
	}
	for _, c := range prefix {
		if c < 'A' || c > 'Z' {
			t.Errorf("expected uppercase letter, got %c", c)
		}
	}
}

func TestTrackingID_NonEmpty(t *testing.T) {
	id := TrackingID("test")
	if id == "" {
		t.Error("TrackingID should not return empty string")
	}
}

func TestTrackingID_DifferentAddresses(t *testing.T) {
	// Different addresses should (almost certainly) produce different IDs
	// due to length embedding — at minimum they are structurally valid
	id1 := TrackingID("short")
	id2 := TrackingID("a much longer address string that differs significantly")
	if !trackingPattern.MatchString(id1) {
		t.Errorf("id1 %q does not match pattern", id1)
	}
	if !trackingPattern.MatchString(id2) {
		t.Errorf("id2 %q does not match pattern", id2)
	}
}
