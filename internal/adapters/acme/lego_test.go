package acme

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

// A generated account key must round-trip: re-parsing its PEM succeeds and the
// supplied PEM is echoed back unchanged (so the account stays stable across
// renewals).
func TestLoadOrGenerateAccountKeyRoundTrip(t *testing.T) {
	key1, pem1, err := loadOrGenerateAccountKey("")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if key1 == nil || pem1 == "" {
		t.Fatal("generate returned empty key/pem")
	}
	key2, pem2, err := loadOrGenerateAccountKey(pem1)
	if err != nil {
		t.Fatalf("re-parse generated pem: %v", err)
	}
	if key2 == nil {
		t.Fatal("re-parse returned nil key")
	}
	if pem2 != pem1 {
		t.Fatalf("supplied PEM not echoed back:\n%q\nvs\n%q", pem1, pem2)
	}
}

func TestLoadAccountKeyRejectsGarbage(t *testing.T) {
	if _, _, err := loadOrGenerateAccountKey("not a valid pem"); err == nil {
		t.Fatal("garbage account key must error")
	}
}

func TestLeafInfoExtractsValidityAndFingerprint(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	nb := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	na := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "test.example"},
		NotBefore:    nb,
		NotAfter:     na,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	gotNB, gotNA, fp, err := leafInfo(certPEM)
	if err != nil {
		t.Fatalf("leafInfo: %v", err)
	}
	if !gotNB.Equal(nb) || !gotNA.Equal(na) {
		t.Fatalf("validity = %v..%v, want %v..%v", gotNB, gotNA, nb, na)
	}
	sum := sha256.Sum256(der)
	wantFP := hex.EncodeToString(sum[:])
	if fp != wantFP {
		t.Fatalf("fingerprint = %q, want %q", fp, wantFP)
	}
	if len(fp) != 64 {
		t.Fatalf("fingerprint not 64 hex chars: %q", fp)
	}
}

func TestLeafInfoRejectsNonPEM(t *testing.T) {
	if _, _, _, err := leafInfo([]byte("garbage")); err == nil {
		t.Fatal("non-PEM cert must error")
	}
}

// An unknown provider must be reported as unsupported (the registry lookup
// fails before any env is touched).
func TestDNSProviderUnknownIsUnsupported(t *testing.T) {
	var i Issuer
	_, _, err := i.dnsProvider(ports.ACMERequest{DNSProvider: "definitely-not-real-xyz"})
	if err == nil {
		t.Fatal("unknown provider must error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("want an 'unsupported' error, got: %v", err)
	}
}

// A known provider whose constructor fails (missing creds) must still leave the
// environment clean — the cleanup unsets exactly the keys we exported, so a
// failed issuance can't leak credentials into the next one.
func TestDNSProviderSetsThenCleansEnvOnFactoryFailure(t *testing.T) {
	const probe = "PSP_ACME_TEST_PROBE_ENV"
	_ = os.Unsetenv(probe)
	var i Issuer
	_, _, err := i.dnsProvider(ports.ACMERequest{
		DNSProvider:    "cloudflare", // factory errors without CF_* creds
		DNSCredentials: map[string]string{probe: "leaked"},
	})
	if err == nil {
		t.Fatal("cloudflare without creds must error")
	}
	if v := os.Getenv(probe); v != "" {
		t.Fatalf("env not cleaned after factory failure: %q", v)
	}
}

func TestSupportedProvidersIncludesCommonAndGenericFallbacks(t *testing.T) {
	set := map[string]bool{}
	for _, p := range SupportedProviders() {
		set[p] = true
	}
	for _, want := range []string{"cloudflare", "alidns", "dnspod", "route53", "exec", "httpreq"} {
		if !set[want] {
			t.Fatalf("provider %q missing from SupportedProviders()", want)
		}
	}
}

// Drift guard: every curated (non-generic) provider with a factory MUST carry a
// field schema, and every schema MUST map to a real factory — so the admin UI
// never falls back to the raw KEY/VALUE editor for a built-in vendor, and a stale
// schema can't reference a removed provider.
func TestProviderMetaCoversFactories(t *testing.T) {
	generic := map[string]bool{"exec": true, "httpreq": true}
	for code := range providerFactories {
		if generic[code] {
			if _, ok := providerMeta[code]; ok {
				t.Errorf("generic provider %q must NOT have a fixed schema (it's free-form KV)", code)
			}
			continue
		}
		meta, ok := providerMeta[code]
		if !ok {
			t.Errorf("provider %q has a factory but no providerMeta schema", code)
			continue
		}
		if len(meta.Fields) == 0 {
			t.Errorf("provider %q schema has no fields", code)
		}
		for _, f := range meta.Fields {
			if f.Key == "" || f.Label == "" {
				t.Errorf("provider %q has a field with empty key or label: %+v", code, f)
			}
		}
	}
	for code := range providerMeta {
		if _, ok := providerFactories[code]; !ok {
			t.Errorf("providerMeta has %q but no matching factory", code)
		}
	}
}

// Every curated provider surfaces with a schema; generics surface as Custom.
func TestSupportedProviderInfosShape(t *testing.T) {
	for _, info := range SupportedProviderInfos() {
		if info.Name == "" || info.Label == "" {
			t.Errorf("provider info missing name/label: %+v", info)
		}
		if info.Custom {
			if len(info.Fields) != 0 {
				t.Errorf("custom provider %q should carry no fields", info.Name)
			}
		} else if len(info.Fields) == 0 {
			t.Errorf("curated provider %q should carry fields", info.Name)
		}
	}
}
