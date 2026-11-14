package shipping

import (
	"fmt"
	"strings"
)

// countryList is every ISO 3166-1 alpha-2 code with its common English name,
// plus XK (Kosovo, user-assigned, used by the EU and most carriers). The
// frontend's country field is free text, so names must resolve, not only codes.
// TestCountryListMatchesISOCodes keeps this list and iso3166Alpha2 in sync.
var countryList = []struct{ Code, Name string }{
	{"AD", "Andorra"}, {"AE", "United Arab Emirates"}, {"AF", "Afghanistan"}, {"AG", "Antigua and Barbuda"},
	{"AI", "Anguilla"}, {"AL", "Albania"}, {"AM", "Armenia"}, {"AO", "Angola"}, {"AQ", "Antarctica"},
	{"AR", "Argentina"}, {"AS", "American Samoa"}, {"AT", "Austria"}, {"AU", "Australia"}, {"AW", "Aruba"},
	{"AX", "Åland Islands"}, {"AZ", "Azerbaijan"},
	{"BA", "Bosnia and Herzegovina"}, {"BB", "Barbados"}, {"BD", "Bangladesh"}, {"BE", "Belgium"},
	{"BF", "Burkina Faso"}, {"BG", "Bulgaria"}, {"BH", "Bahrain"}, {"BI", "Burundi"}, {"BJ", "Benin"},
	{"BL", "Saint Barthélemy"}, {"BM", "Bermuda"}, {"BN", "Brunei"}, {"BO", "Bolivia"},
	{"BQ", "Bonaire, Sint Eustatius and Saba"}, {"BR", "Brazil"}, {"BS", "Bahamas"}, {"BT", "Bhutan"},
	{"BV", "Bouvet Island"}, {"BW", "Botswana"}, {"BY", "Belarus"}, {"BZ", "Belize"},
	{"CA", "Canada"}, {"CC", "Cocos (Keeling) Islands"}, {"CD", "Democratic Republic of the Congo"},
	{"CF", "Central African Republic"}, {"CG", "Republic of the Congo"}, {"CH", "Switzerland"},
	{"CI", "Côte d'Ivoire"}, {"CK", "Cook Islands"}, {"CL", "Chile"}, {"CM", "Cameroon"}, {"CN", "China"},
	{"CO", "Colombia"}, {"CR", "Costa Rica"}, {"CU", "Cuba"}, {"CV", "Cabo Verde"}, {"CW", "Curaçao"},
	{"CX", "Christmas Island"}, {"CY", "Cyprus"}, {"CZ", "Czechia"},
	{"DE", "Germany"}, {"DJ", "Djibouti"}, {"DK", "Denmark"}, {"DM", "Dominica"}, {"DO", "Dominican Republic"},
	{"DZ", "Algeria"},
	{"EC", "Ecuador"}, {"EE", "Estonia"}, {"EG", "Egypt"}, {"EH", "Western Sahara"}, {"ER", "Eritrea"},
	{"ES", "Spain"}, {"ET", "Ethiopia"},
	{"FI", "Finland"}, {"FJ", "Fiji"}, {"FK", "Falkland Islands"}, {"FM", "Micronesia"}, {"FO", "Faroe Islands"},
	{"FR", "France"},
	{"GA", "Gabon"}, {"GB", "United Kingdom"}, {"GD", "Grenada"}, {"GE", "Georgia"}, {"GF", "French Guiana"},
	{"GG", "Guernsey"}, {"GH", "Ghana"}, {"GI", "Gibraltar"}, {"GL", "Greenland"}, {"GM", "Gambia"},
	{"GN", "Guinea"}, {"GP", "Guadeloupe"}, {"GQ", "Equatorial Guinea"}, {"GR", "Greece"},
	{"GS", "South Georgia and the South Sandwich Islands"}, {"GT", "Guatemala"}, {"GU", "Guam"},
	{"GW", "Guinea-Bissau"}, {"GY", "Guyana"},
	{"HK", "Hong Kong"}, {"HM", "Heard Island and McDonald Islands"}, {"HN", "Honduras"}, {"HR", "Croatia"},
	{"HT", "Haiti"}, {"HU", "Hungary"},
	{"ID", "Indonesia"}, {"IE", "Ireland"}, {"IL", "Israel"}, {"IM", "Isle of Man"}, {"IN", "India"},
	{"IO", "British Indian Ocean Territory"}, {"IQ", "Iraq"}, {"IR", "Iran"}, {"IS", "Iceland"}, {"IT", "Italy"},
	{"JE", "Jersey"}, {"JM", "Jamaica"}, {"JO", "Jordan"}, {"JP", "Japan"},
	{"KE", "Kenya"}, {"KG", "Kyrgyzstan"}, {"KH", "Cambodia"}, {"KI", "Kiribati"}, {"KM", "Comoros"},
	{"KN", "Saint Kitts and Nevis"}, {"KP", "North Korea"}, {"KR", "South Korea"}, {"KW", "Kuwait"},
	{"KY", "Cayman Islands"}, {"KZ", "Kazakhstan"},
	{"LA", "Laos"}, {"LB", "Lebanon"}, {"LC", "Saint Lucia"}, {"LI", "Liechtenstein"}, {"LK", "Sri Lanka"},
	{"LR", "Liberia"}, {"LS", "Lesotho"}, {"LT", "Lithuania"}, {"LU", "Luxembourg"}, {"LV", "Latvia"},
	{"LY", "Libya"},
	{"MA", "Morocco"}, {"MC", "Monaco"}, {"MD", "Moldova"}, {"ME", "Montenegro"}, {"MF", "Saint Martin"},
	{"MG", "Madagascar"}, {"MH", "Marshall Islands"}, {"MK", "North Macedonia"}, {"ML", "Mali"},
	{"MM", "Myanmar"}, {"MN", "Mongolia"}, {"MO", "Macao"}, {"MP", "Northern Mariana Islands"},
	{"MQ", "Martinique"}, {"MR", "Mauritania"}, {"MS", "Montserrat"}, {"MT", "Malta"}, {"MU", "Mauritius"},
	{"MV", "Maldives"}, {"MW", "Malawi"}, {"MX", "Mexico"}, {"MY", "Malaysia"}, {"MZ", "Mozambique"},
	{"NA", "Namibia"}, {"NC", "New Caledonia"}, {"NE", "Niger"}, {"NF", "Norfolk Island"}, {"NG", "Nigeria"},
	{"NI", "Nicaragua"}, {"NL", "Netherlands"}, {"NO", "Norway"}, {"NP", "Nepal"}, {"NR", "Nauru"},
	{"NU", "Niue"}, {"NZ", "New Zealand"},
	{"OM", "Oman"},
	{"PA", "Panama"}, {"PE", "Peru"}, {"PF", "French Polynesia"}, {"PG", "Papua New Guinea"},
	{"PH", "Philippines"}, {"PK", "Pakistan"}, {"PL", "Poland"}, {"PM", "Saint Pierre and Miquelon"},
	{"PN", "Pitcairn Islands"}, {"PR", "Puerto Rico"}, {"PS", "Palestine"}, {"PT", "Portugal"},
	{"PW", "Palau"}, {"PY", "Paraguay"},
	{"QA", "Qatar"},
	{"RE", "Réunion"}, {"RO", "Romania"}, {"RS", "Serbia"}, {"RU", "Russia"}, {"RW", "Rwanda"},
	{"SA", "Saudi Arabia"}, {"SB", "Solomon Islands"}, {"SC", "Seychelles"}, {"SD", "Sudan"}, {"SE", "Sweden"},
	{"SG", "Singapore"}, {"SH", "Saint Helena"}, {"SI", "Slovenia"}, {"SJ", "Svalbard and Jan Mayen"},
	{"SK", "Slovakia"}, {"SL", "Sierra Leone"}, {"SM", "San Marino"}, {"SN", "Senegal"}, {"SO", "Somalia"},
	{"SR", "Suriname"}, {"SS", "South Sudan"}, {"ST", "São Tomé and Príncipe"}, {"SV", "El Salvador"},
	{"SX", "Sint Maarten"}, {"SY", "Syria"}, {"SZ", "Eswatini"},
	{"TC", "Turks and Caicos Islands"}, {"TD", "Chad"}, {"TF", "French Southern Territories"}, {"TG", "Togo"},
	{"TH", "Thailand"}, {"TJ", "Tajikistan"}, {"TK", "Tokelau"}, {"TL", "Timor-Leste"}, {"TM", "Turkmenistan"},
	{"TN", "Tunisia"}, {"TO", "Tonga"}, {"TR", "Türkiye"}, {"TT", "Trinidad and Tobago"}, {"TV", "Tuvalu"},
	{"TW", "Taiwan"}, {"TZ", "Tanzania"},
	{"UA", "Ukraine"}, {"UG", "Uganda"}, {"UM", "United States Minor Outlying Islands"},
	{"US", "United States"}, {"UY", "Uruguay"}, {"UZ", "Uzbekistan"},
	{"VA", "Vatican City"}, {"VC", "Saint Vincent and the Grenadines"}, {"VE", "Venezuela"},
	{"VG", "British Virgin Islands"}, {"VI", "US Virgin Islands"}, {"VN", "Vietnam"}, {"VU", "Vanuatu"},
	{"WF", "Wallis and Futuna"}, {"WS", "Samoa"},
	{"YE", "Yemen"}, {"YT", "Mayotte"},
	{"ZA", "South Africa"}, {"ZM", "Zambia"}, {"ZW", "Zimbabwe"},
	{"XK", "Kosovo"},
}

// countryAliases are alternative spellings, official long names and common
// abbreviations that people type into a free-text field. The older entries
// in countryNames (address.go) are merged in as well.
var countryAliases = map[string]string{
	"viet nam": "VN", "hongkong": "HK", "hong kong sar": "HK", "macau": "MO", "macao sar": "MO",
	"ivory coast": "CI", "cote divoire": "CI",
	"czech republic": "CZ", "swaziland": "SZ", "burma": "MM", "east timor": "TL",
	"cape verde": "CV", "the bahamas": "BS", "the gambia": "GM", "gambia the": "GM",
	"brunei darussalam": "BN", "republic of korea": "KR", "korea south": "KR", "republic of south korea": "KR",
	"democratic peoples republic of korea": "KP", "korea north": "KP", "dprk": "KP",
	"lao pdr": "LA", "lao peoples democratic republic": "LA",
	"russian federation": "RU", "syrian arab republic": "SY", "islamic republic of iran": "IR",
	"republic of china": "TW", "chinese taipei": "TW", "taiwan roc": "TW", "prc": "CN", "peoples republic of china": "CN",
	"united republic of tanzania": "TZ", "bolivarian republic of venezuela": "VE", "plurinational state of bolivia": "BO",
	"republic of moldova": "MD", "state of palestine": "PS", "palestinian territories": "PS", "west bank": "PS", "gaza": "PS",
	"democratic republic of congo": "CD", "dr congo": "CD", "drc": "CD", "congo kinshasa": "CD", "congo democratic republic": "CD",
	"congo brazzaville": "CG", "congo republic": "CG", "congo": "CG",
	"holy see": "VA", "vatican": "VA", "the vatican": "VA",
	"micronesia federated states of": "FM", "federated states of micronesia": "FM",
	"falkland islands malvinas": "FK", "malvinas": "FK",
	"sao tome and principe": "ST", "st kitts and nevis": "KN", "st lucia": "LC", "st helena": "SH",
	"st vincent and the grenadines": "VC", "st martin": "MF", "st barthelemy": "BL", "st pierre and miquelon": "PM",
	"reunion": "RE", "aland islands": "AX", "aland": "AX", "curacao": "CW", "turkiye": "TR",
	"timor leste": "TL", "svalbard": "SJ", "pitcairn": "PN", "us virgin islands": "VI", "united states virgin islands": "VI",
	"virgin islands us": "VI", "virgin islands british": "VG", "u s virgin islands": "VI",
	"bonaire": "BQ", "sint eustatius": "BQ", "saba": "BQ",
	"republic of kosovo": "XK", "kosova": "XK",
	"republic of ireland": "IE", "eire": "IE", "the philippines": "PH", "the czech republic": "CZ",
	"republic of north macedonia": "MK", "fyrom": "MK", "belarus": "BY", "byelorussia": "BY",
	"united arab emirates uae": "AE", "emirates": "AE", "ksa": "SA",
	"new zealand aotearoa": "NZ", "aotearoa": "NZ",
}

// extraCodes are accepted as a two-letter code although they are not assigned
// by ISO 3166-1: XK (Kosovo).
var extraCodes = map[string]bool{"XK": true}

// countryIndex maps a normalised country name or alias to its ISO code.
var countryIndex = func() map[string]string {
	m := make(map[string]string, 600)
	for _, c := range countryList {
		m[normaliseCountryInput(c.Name)] = c.Code
	}
	for k, v := range countryNames {
		m[normaliseCountryInput(k)] = v
	}
	for k, v := range countryAliases {
		m[normaliseCountryInput(k)] = v
	}
	return m
}()

// diacritics folds the accented letters found in country names so that
// "Côte d'Ivoire" and "Cote d'Ivoire" match.
var diacritics = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ä", "a", "ã", "a", "å", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "î", "i", "ï", "i", "ı", "i",
	"ó", "o", "ô", "o", "ö", "o", "õ", "o",
	"ú", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n", "ş", "s", "ğ", "g",
)

// maxCountryEcho caps how much of a free-text country is repeated in an error.
const maxCountryEcho = 60

// ErrCountryRequired is returned for a blank country.
var ErrCountryRequired = fmt.Errorf("country is required")

// ResolveCountry is the single country check used by both GetQuote and
// ShipOrder: it turns a free-text country (ISO 3166-1 alpha-2 code in any
// case, or an English name or common alias) into the upper-case code. A blank
// input yields ErrCountryRequired, an unknown one a readable error that tells
// the caller what to type.
func ResolveCountry(country string) (string, error) {
	if strings.TrimSpace(country) == "" {
		return "", ErrCountryRequired
	}
	if code, ok := NormalizeCountry(country); ok {
		return code, nil
	}
	shown := []rune(strings.TrimSpace(country))
	if len(shown) > maxCountryEcho {
		shown = append(shown[:maxCountryEcho], '…')
	}
	return "", fmt.Errorf("country %q is not an ISO 3166-1 alpha-2 code or a known country name (use e.g. \"United States\" or \"US\")", string(shown))
}
