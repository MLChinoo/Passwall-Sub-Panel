// Package geoip resolves IPs to a GeoLocation using a local MaxMind-format
// (.mmdb) database — fully offline, no external calls, no caching needed (the
// memory-mapped DB IS the lookup). One Reader reads either schema:
//
//   - MaxMind GeoLite2 / GeoIP2 / db-ip Lite: nested objects, where the top
//     "country" key is a map {iso_code, names:{en:...}}, plus "city" and
//     "subdivisions" for city-level databases.
//   - ipinfo Lite: flat string fields, where "country" is the country NAME
//     (a string) and "country_code" is the ISO code; country-level only.
//
// Because the two schemas collide on the "country" key (map vs string), we
// decode each record into a generic map and branch on the runtime type rather
// than a fixed struct — this also tolerates any future MaxMind-compatible DB.
package geoip

import (
	"net"
	"strings"

	maxminddb "github.com/oschwald/maxminddb-golang"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

// Reader wraps an open .mmdb database. Safe for concurrent Lookup (the
// underlying maxminddb.Reader is thread-safe); the geo service owns its
// lifecycle (open/close/reload).
type Reader struct {
	db *maxminddb.Reader
}

// DBInfo describes a loaded database for the admin status view.
type DBInfo struct {
	Type        string // Metadata.DatabaseType, e.g. "GeoLite2-City" / "ipinfo lite.mmdb"
	BuildEpoch  uint   // Unix seconds the DB was built (last update)
	Granularity string // "city" or "country"
}

// Open opens an .mmdb file.
func Open(path string) (*Reader, error) {
	db, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	return &Reader{db: db}, nil
}

// Close releases the database.
func (r *Reader) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// Info returns metadata for the status view.
func (r *Reader) Info() DBInfo {
	m := r.db.Metadata
	return DBInfo{
		Type:        m.DatabaseType,
		BuildEpoch:  m.BuildEpoch,
		Granularity: granularityOf(m.DatabaseType),
	}
}

func granularityOf(dbType string) string {
	if strings.Contains(strings.ToLower(dbType), "city") {
		return "city"
	}
	return "country"
}

// Lookup resolves one IP. Returns a zero GeoLocation (no error) for an
// unparseable / private / unmapped IP so callers can treat "unknown" uniformly.
func (r *Reader) Lookup(ip string) (domain.GeoLocation, error) {
	if r == nil || r.db == nil {
		return domain.GeoLocation{}, nil
	}
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil || !IsResolvable(ip) {
		return domain.GeoLocation{}, nil
	}
	var rec map[string]any
	if err := r.db.Lookup(parsed, &rec); err != nil {
		return domain.GeoLocation{}, err
	}
	return mapRecord(rec), nil
}

// IsResolvable reports whether ip is a routable public address worth looking
// up. Private / loopback / link-local / unspecified addresses have no useful
// geolocation.
func IsResolvable(ip string) bool {
	p := net.ParseIP(strings.TrimSpace(ip))
	if p == nil {
		return false
	}
	return !(p.IsLoopback() || p.IsPrivate() || p.IsUnspecified() ||
		p.IsLinkLocalUnicast() || p.IsLinkLocalMulticast())
}

// mapRecord flattens a decoded mmdb record (either schema) into a GeoLocation.
func mapRecord(rec map[string]any) domain.GeoLocation {
	if rec == nil {
		return domain.GeoLocation{}
	}
	var loc domain.GeoLocation
	switch c := rec["country"].(type) {
	case map[string]any: // MaxMind / GeoLite2 / db-ip schema
		loc.CountryCode = asString(c["iso_code"])
		loc.Country = enName(c["names"])
		loc.City = enName(mapOf(rec["city"])["names"])
		if subs, ok := rec["subdivisions"].([]any); ok && len(subs) > 0 {
			loc.Region = enName(mapOf(subs[0])["names"])
		}
	case string: // ipinfo Lite schema (country is the name string)
		loc.Country = c
		loc.CountryCode = asString(rec["country_code"])
		// ipinfo Lite is country-level: no city/region.
	default:
		// Some ipinfo variants key the code as country_code with no "country".
		loc.CountryCode = asString(rec["country_code"])
		loc.Country = asString(rec["country_name"])
	}
	loc.CountryCode = strings.ToUpper(strings.TrimSpace(loc.CountryCode))
	return loc
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func mapOf(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// enName extracts the English display name from a maxminddb "names" map,
// falling back to any available language so a non-en-only DB still shows
// something.
func enName(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	if s := asString(m["en"]); s != "" {
		return s
	}
	for _, val := range m {
		if s := asString(val); s != "" {
			return s
		}
	}
	return ""
}
