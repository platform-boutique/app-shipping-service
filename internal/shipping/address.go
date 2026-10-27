package shipping

import (
	"errors"
	"fmt"
	"strings"

	pb "shipping-service/proto"
)

// iso3166Alpha2 is the full list of officially assigned ISO 3166-1 alpha-2 codes.
const iso3166Alpha2 = "" +
	"AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ " +
	"BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ " +
	"CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ " +
	"DE DJ DK DM DO DZ " +
	"EC EE EG EH ER ES ET " +
	"FI FJ FK FM FO FR " +
	"GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GS GT GU GW GY " +
	"HK HM HN HR HT HU " +
	"ID IE IL IM IN IO IQ IR IS IT " +
	"JE JM JO JP " +
	"KE KG KH KI KM KN KP KR KW KY KZ " +
	"LA LB LC LI LK LR LS LT LU LV LY " +
	"MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT MU MV MW MX MY MZ " +
	"NA NC NE NF NG NI NL NO NP NR NU NZ " +
	"OM " +
	"PA PE PF PG PH PK PL PM PN PR PS PT PW PY " +
	"QA " +
	"RE RO RS RU RW " +
	"SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST SV SX SY SZ " +
	"TC TD TF TG TH TJ TK TL TM TN TO TR TT TV TW TZ " +
	"UA UG UM US UY UZ " +
	"VA VC VE VG VI VN VU " +
	"WF WS " +
	"YE YT " +
	"ZA ZM ZW"

var isoCodes = func() map[string]bool {
	m := make(map[string]bool)
	for _, c := range strings.Fields(iso3166Alpha2) {
		m[c] = true
	}
	return m
}()

// countryNames maps common English (and a few native) country names and
// abbreviations, in normalised form (lower case, no dots, single spaces), to
// their ISO code. The frontend's checkout form sends a free-text name
// ("United States" by default), so the common ones must resolve.
var countryNames = map[string]string{
	"united states": "US", "united states of america": "US", "usa": "US", "us": "US", "america": "US",
	"canada": "CA",
	"mexico": "MX", "méxico": "MX",
	"united kingdom": "GB", "uk": "GB", "great britain": "GB", "britain": "GB",
	"england": "GB", "scotland": "GB", "wales": "GB", "northern ireland": "GB",
	"germany": "DE", "deutschland": "DE",
	"france": "FR",
	"italy":  "IT", "italia": "IT",
	"spain": "ES", "españa": "ES", "espana": "ES",
	"netherlands": "NL", "the netherlands": "NL", "holland": "NL",
	"belgium": "BE",
	"austria": "AT", "österreich": "AT", "osterreich": "AT",
	"switzerland": "CH", "schweiz": "CH", "suisse": "CH",
	"ireland":  "IE",
	"portugal": "PT",
	"poland":   "PL", "polska": "PL",
	"sweden":         "SE",
	"norway":         "NO",
	"denmark":        "DK",
	"finland":        "FI",
	"iceland":        "IS",
	"greece":         "GR",
	"czech republic": "CZ", "czechia": "CZ",
	"slovakia":        "SK",
	"hungary":         "HU",
	"romania":         "RO",
	"bulgaria":        "BG",
	"croatia":         "HR",
	"slovenia":        "SI",
	"serbia":          "RS",
	"estonia":         "EE",
	"latvia":          "LV",
	"lithuania":       "LT",
	"luxembourg":      "LU",
	"malta":           "MT",
	"cyprus":          "CY",
	"ukraine":         "UA",
	"north macedonia": "MK", "macedonia": "MK",
	"albania":                "AL",
	"bosnia and herzegovina": "BA",
	"montenegro":             "ME",
	"moldova":                "MD",
	"monaco":                 "MC",
	"liechtenstein":          "LI",
	"andorra":                "AD",
	"san marino":             "SM",
	"australia":              "AU",
	"new zealand":            "NZ",
	"japan":                  "JP",
	"china":                  "CN",
	"india":                  "IN",
	"brazil":                 "BR",
	"argentina":              "AR",
	"chile":                  "CL",
	"colombia":               "CO",
	"peru":                   "PE",
	"south africa":           "ZA",
	"south korea":            "KR", "korea": "KR",
	"singapore": "SG",
	"turkey":    "TR", "türkiye": "TR", "turkiye": "TR",
	"russia":               "RU",
	"israel":               "IL",
	"united arab emirates": "AE", "uae": "AE",
	"saudi arabia": "SA",
	"egypt":        "EG",
	"nigeria":      "NG",
	"kenya":        "KE",
}

// numericPostcode lists the countries whose postal codes are purely numeric,
// so they fit the proto's int32 zip_code. For these, ShipOrder requires
// zip_code > 0. Every other country (alphanumeric codes such as GB, CA, NL, or
// no postal codes at all) accepts any zip_code >= 0, including 0.
var numericPostcode = map[string]bool{
	"US": true, "MX": true, "DE": true, "FR": true, "IT": true, "ES": true,
	"AT": true, "CH": true, "BE": true, "DK": true, "NO": true, "SE": true,
	"FI": true, "AU": true,
}

// normaliseCountryInput lower-cases, drops dots and apostrophes, folds accents
// and turns hyphens, commas, parentheses and "&" into single spaces.
func normaliseCountryInput(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer(".", "", "'", "", "\u2019", "", "-", " ", ",", " ", "(", " ", ")", " ", "&", " and ").Replace(s)
	s = diacritics.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// NormalizeCountry turns a country given as an ISO 3166-1 alpha-2 code (any
// case) or a known name into its upper-case ISO code. ok is false when the
// input is neither.
func NormalizeCountry(country string) (code string, ok bool) {
	n := normaliseCountryInput(country)
	if n == "" {
		return "", false
	}
	if c, found := countryIndex[n]; found {
		return c, true
	}
	if up := strings.ToUpper(n); len(up) == 2 && (isoCodes[up] || extraCodes[up]) {
		return up, true
	}
	return "", false
}

// ErrAddressRequired is returned when a request carries no address.
var ErrAddressRequired = errors.New("address is required")

// ValidateAddress checks an address for ShipOrder and returns the normalised
// ISO country code. Rules (deliberately lenient):
//   - street_address, city and country must be non-blank;
//   - country must be an ISO 3166-1 alpha-2 code or a known name (ResolveCountry,
//     the same check GetQuote applies when a country is given);
//   - zip_code must never be negative, and must be > 0 for countries with
//     purely numeric postal codes (see numericPostcode).
//
// state is not validated.
func ValidateAddress(a *pb.Address) (string, error) {
	if a == nil {
		return "", ErrAddressRequired
	}
	var problems []string
	if strings.TrimSpace(a.GetStreetAddress()) == "" {
		problems = append(problems, "street_address is required")
	}
	if strings.TrimSpace(a.GetCity()) == "" {
		problems = append(problems, "city is required")
	}
	code, err := ResolveCountry(a.GetCountry())
	if err != nil {
		problems = append(problems, err.Error())
	}
	if a.GetZipCode() < 0 {
		problems = append(problems, "zip_code must not be negative")
	} else if code != "" && numericPostcode[code] && a.GetZipCode() == 0 {
		problems = append(problems, fmt.Sprintf("zip_code is required for country %s", code))
	}
	if len(problems) > 0 {
		return "", fmt.Errorf("invalid address: %s", strings.Join(problems, "; "))
	}
	return code, nil
}
