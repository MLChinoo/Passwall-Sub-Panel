package geo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
	"time"

	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

func gz(b []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(b)
	_ = w.Close()
	return buf.Bytes()
}

func targz(name string, content []byte) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg})
	_, _ = tw.Write(content)
	_ = tw.Close()
	_ = gw.Close()
	return buf.Bytes()
}

func TestExtractMMDB(t *testing.T) {
	// Plain bytes pass through.
	if got, err := extractMMDB([]byte("RAW-MMDB")); err != nil || string(got) != "RAW-MMDB" {
		t.Fatalf("plain: got %q err %v", got, err)
	}
	// gzip'd .mmdb (DB-IP / ipinfo .gz) → inner bytes.
	if got, err := extractMMDB(gz([]byte("INNER-MMDB"))); err != nil || string(got) != "INNER-MMDB" {
		t.Fatalf("gzip: got %q err %v", got, err)
	}
	// MaxMind .tar.gz with the .mmdb nested in a dated folder → inner bytes.
	blob := targz("GeoLite2-City_20260601/GeoLite2-City.mmdb", []byte("TAR-MMDB"))
	if got, err := extractMMDB(blob); err != nil || string(got) != "TAR-MMDB" {
		t.Fatalf("tar.gz: got %q err %v", got, err)
	}
	// tar.gz with a non-.mmdb entry → falls back to treating gunzipped data as
	// the file (here it's a tar, no .mmdb found → returns the tar bytes, which a
	// later geoip.Open would reject — acceptable; the point is no crash).
	if _, err := extractMMDB(targz("README.txt", []byte("hi"))); err != nil {
		t.Fatalf("tar.gz without mmdb should not error at extract stage: %v", err)
	}
}

func TestCandidateURLs(t *testing.T) {
	// maxmind requires a license key.
	if _, _, err := candidateURLs(ports.UISettings{GeoIPUpdateSource: "maxmind"}); err == nil {
		t.Fatal("maxmind without token must error")
	}
	urls, target, err := candidateURLs(ports.UISettings{
		GeoIPUpdateSource: "maxmind", GeoIPUpdateToken: "LIC123", GeoIPUpdateEdition: "GeoLite2-City",
	})
	if err != nil || len(urls) != 1 ||
		!strings.Contains(urls[0], "download.maxmind.com") ||
		!strings.Contains(urls[0], "edition_id=GeoLite2-City") ||
		!strings.Contains(urls[0], "license_key=LIC123") {
		t.Fatalf("maxmind url = %v (%v)", urls, err)
	}
	if target != "GeoLite2-City.mmdb" {
		t.Fatalf("maxmind target = %q", target)
	}

	// ipinfo requires a token.
	if _, _, err := candidateURLs(ports.UISettings{GeoIPUpdateSource: "ipinfo"}); err == nil {
		t.Fatal("ipinfo without token must error")
	}
	urls, target, _ = candidateURLs(ports.UISettings{GeoIPUpdateSource: "ipinfo", GeoIPUpdateToken: "tok"})
	if len(urls) != 1 || !strings.Contains(urls[0], "ipinfo.io/data/ipinfo_lite.mmdb") || target != "ipinfo-lite.mmdb" {
		t.Fatalf("ipinfo url/target = %v / %q", urls, target)
	}

	// dbip returns current + previous month (month-stamped, no stable latest).
	urls, target, _ = candidateURLs(ports.UISettings{GeoIPUpdateSource: "dbip"})
	if len(urls) != 2 || target != "dbip-city-lite.mmdb" {
		t.Fatalf("dbip urls/target = %v / %q", urls, target)
	}

	// custom requires a URL; target derived from basename.
	if _, _, err := candidateURLs(ports.UISettings{GeoIPUpdateSource: "custom"}); err == nil {
		t.Fatal("custom without url must error")
	}
	urls, target, _ = candidateURLs(ports.UISettings{GeoIPUpdateSource: "custom", GeoIPUpdateURL: "https://x.test/my-geo.mmdb"})
	if len(urls) != 1 || target != "my-geo.mmdb" {
		t.Fatalf("custom urls/target = %v / %q", urls, target)
	}

	// unknown source errors.
	if _, _, err := candidateURLs(ports.UISettings{GeoIPUpdateSource: "bogus"}); err == nil {
		t.Fatal("unknown source must error")
	}
}

func TestCustomTarget(t *testing.T) {
	cases := map[string]string{
		"https://x.test/foo.mmdb":          "foo.mmdb",
		"https://x.test/foo.mmdb.gz":       "foo.mmdb",
		"https://x.test/bar.tar.gz":        "custom.mmdb", // stripped to "bar" → not .mmdb
		"https://x.test/db.mmdb?token=abc": "db.mmdb",
		"https://x.test/":                  "custom.mmdb",
	}
	for in, want := range cases {
		if got := customTarget(in); got != want {
			t.Errorf("customTarget(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDBIPURL(t *testing.T) {
	got := dbipURL(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if got != "https://download.db-ip.com/free/dbip-city-lite-2026-06.mmdb.gz" {
		t.Fatalf("dbipURL = %q", got)
	}
}
