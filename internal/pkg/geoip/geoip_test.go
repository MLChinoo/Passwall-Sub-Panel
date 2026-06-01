package geoip

import "testing"

// mapRecord is the heart of the dual-schema support: MaxMind's "country" is a
// nested object, ipinfo Lite's "country" is a plain string. Both must flatten
// to the same GeoLocation.
func TestMapRecord_MaxMindSchema(t *testing.T) {
	rec := map[string]any{
		"country": map[string]any{
			"iso_code": "HK",
			"names":    map[string]any{"en": "Hong Kong", "zh-CN": "香港"},
		},
		"city": map[string]any{"names": map[string]any{"en": "Central"}},
		"subdivisions": []any{
			map[string]any{"names": map[string]any{"en": "Central and Western"}},
		},
	}
	got := mapRecord(rec)
	if got.CountryCode != "HK" || got.Country != "Hong Kong" {
		t.Fatalf("country = %q/%q, want HK/Hong Kong", got.CountryCode, got.Country)
	}
	if got.City != "Central" {
		t.Fatalf("city = %q, want Central", got.City)
	}
	if got.Region != "Central and Western" {
		t.Fatalf("region = %q, want Central and Western", got.Region)
	}
}

func TestMapRecord_IPinfoSchema(t *testing.T) {
	rec := map[string]any{
		"country":      "Hong Kong", // ipinfo: country is the NAME (string)
		"country_code": "hk",        // lowercase → normalized to upper
		"continent":    "Asia",
	}
	got := mapRecord(rec)
	if got.CountryCode != "HK" {
		t.Fatalf("country_code = %q, want HK (upper-normalized)", got.CountryCode)
	}
	if got.Country != "Hong Kong" {
		t.Fatalf("country = %q, want Hong Kong", got.Country)
	}
	if got.City != "" || got.Region != "" {
		t.Fatalf("ipinfo Lite has no city/region; got city=%q region=%q", got.City, got.Region)
	}
}

func TestMapRecord_EnNameFallback(t *testing.T) {
	// No "en" key — fall back to any available language so a non-en DB still
	// shows something.
	rec := map[string]any{
		"country": map[string]any{"iso_code": "JP", "names": map[string]any{"ja": "日本"}},
	}
	if got := mapRecord(rec).Country; got != "日本" {
		t.Fatalf("country name fallback = %q, want 日本", got)
	}
}

func TestIsResolvable(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":              true,
		"1.1.1.1":              true,
		"2001:4860:4860::8888": true,
		"192.168.1.1":          false, // private
		"10.0.0.5":             false, // private
		"127.0.0.1":            false, // loopback
		"::1":                  false, // loopback
		"169.254.1.1":          false, // link-local
		"0.0.0.0":              false, // unspecified
		"":                     false,
		"not-an-ip":            false,
	}
	for ip, want := range cases {
		if got := IsResolvable(ip); got != want {
			t.Errorf("IsResolvable(%q) = %v, want %v", ip, got, want)
		}
	}
}

func TestGranularityOf(t *testing.T) {
	cases := map[string]string{
		"GeoLite2-City":    "city",
		"GeoIP2-City":      "city",
		"DBIP-City-Lite":   "city",
		"GeoLite2-Country": "country",
		"ipinfo lite.mmdb": "country",
		"":                 "country",
	}
	for typ, want := range cases {
		if got := granularityOf(typ); got != want {
			t.Errorf("granularityOf(%q) = %q, want %q", typ, got, want)
		}
	}
}
