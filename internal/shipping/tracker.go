package shipping

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	pb "shipping-service/proto"
)

// OrderSalt is the canonical text a tracking ID is derived from: the address
// (street, city, state, normalised ISO country code, zip, all trimmed) and the
// items (non-positive quantities dropped, quantities of the same product id
// summed, sorted by product id so the order of the items does not matter).
// The same order content always gives the same salt.
func OrderSalt(a *pb.Address, countryCode string, items []*pb.CartItem) string {
	qty := map[string]int64{}
	for _, it := range items {
		if q := it.GetQuantity(); q > 0 {
			qty[strings.TrimSpace(it.GetProductId())] += int64(q)
		}
	}
	ids := make([]string, 0, len(qty))
	for id := range qty {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var b strings.Builder
	fmt.Fprintf(&b, "%s, %s, %s", strings.TrimSpace(a.GetStreetAddress()), strings.TrimSpace(a.GetCity()), strings.TrimSpace(a.GetState()))
	fmt.Fprintf(&b, "|%s|%d|", countryCode, a.GetZipCode())
	for _, id := range ids {
		fmt.Fprintf(&b, "%s=%d;", id, qty[id])
	}
	return b.String()
}

// TrackingID derives a tracking ID from salt (see OrderSalt). It is
// deterministic: the same salt always yields the same ID, so a retried
// ShipOrder for the same order returns the same tracking ID. Format (unchanged
// from the random version): two letters, "-", len(salt) followed by 3 digits,
// "-", len(salt)/2 followed by 7 digits. Letters and digits are taken from
// the SHA-256 of the salt. Two identical orders to the same address share an
// ID; the ID is not secret and not a security token.
func TrackingID(salt string) string {
	h := sha256.Sum256([]byte(salt))
	letter := func(i int) rune { return rune('A' + h[i]%26) }
	digits := func(from, n int) string {
		buf := make([]byte, n)
		for i := range buf {
			buf[i] = '0' + h[from+i]%10
		}
		return string(buf)
	}
	return fmt.Sprintf("%c%c-%d%s-%d%s",
		letter(0), letter(1),
		len(salt), digits(2, 3),
		len(salt)/2, digits(5, 7),
	)
}
