package geo

import (
	"net/url"
	"strings"
	"testing"
)

// TestRedactURLErr pins that a GeoIP download error never carries the secret
// query (MaxMind license_key / IPinfo token) into logs or the admin status.
func TestRedactURLErr(t *testing.T) {
	raw := "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&license_key=SUPER_SECRET_KEY&suffix=tar.gz"

	if got := redactURL(raw); strings.Contains(got, "SUPER_SECRET_KEY") || strings.Contains(got, "license_key") {
		t.Errorf("redactURL leaked the query: %q", got)
	} else if !strings.Contains(got, "download.maxmind.com") {
		t.Errorf("redactURL dropped the host/path: %q", got)
	}

	// A *url.Error (what net/http returns) must be rebuilt without the URL.
	ue := &url.Error{Op: "Get", URL: raw, Err: errTimeout{}}
	msg := redactURLErr(raw, ue).Error()
	if strings.Contains(msg, "SUPER_SECRET_KEY") || strings.Contains(msg, "license_key") {
		t.Errorf("redactURLErr leaked the token: %q", msg)
	}

	// ipinfo token rides in ?token=
	ipinfo := "https://ipinfo.io/data/ipinfo_lite.mmdb?token=TKN123"
	if got := redactURL(ipinfo); strings.Contains(got, "TKN123") {
		t.Errorf("redactURL leaked ipinfo token: %q", got)
	}
}

type errTimeout struct{}

func (errTimeout) Error() string { return "i/o timeout" }
