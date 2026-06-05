package captcha

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

func TestIssue_ImageRoundTrip(t *testing.T) {
	svc := NewService()
	set := ports.UISettings{CaptchaEnabled: true, CaptchaProvider: ProviderImage}

	ch, err := svc.Issue(context.Background(), set)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if ch == nil || ch.ID == "" {
		t.Fatalf("image provider must issue a challenge, got %+v", ch)
	}
	if !strings.HasPrefix(ch.Image, "data:image/") {
		t.Fatalf("image must be a data URL, got %.30q", ch.Image)
	}

	// White-box: read the stored answer (without clearing) to exercise verify.
	answer := svc.store.Get(ch.ID, false)
	if answer == "" {
		t.Fatal("store must hold the answer for the issued id")
	}
	if ok, err := svc.Verify(context.Background(), set, Response{ChallengeID: ch.ID, Answer: answer}); err != nil || !ok {
		t.Fatalf("correct answer must verify: ok=%v err=%v", ok, err)
	}
	// Already cleared by the successful verify → a replay fails.
	if ok, _ := svc.Verify(context.Background(), set, Response{ChallengeID: ch.ID, Answer: answer}); ok {
		t.Fatal("a verified captcha must not be replayable")
	}
}

func TestVerify_ImageWrongAnswer(t *testing.T) {
	svc := NewService()
	set := ports.UISettings{CaptchaEnabled: true, CaptchaProvider: ProviderImage}
	ch, _ := svc.Issue(context.Background(), set)
	if ok, _ := svc.Verify(context.Background(), set, Response{ChallengeID: ch.ID, Answer: "definitely-wrong"}); ok {
		t.Fatal("wrong answer must not verify")
	}
	// Empty answer → normal failure, no error.
	if ok, err := svc.Verify(context.Background(), set, Response{ChallengeID: ch.ID, Answer: ""}); ok || err != nil {
		t.Fatalf("empty answer = (%v, %v), want (false, nil)", ok, err)
	}
}

func TestIssue_TokenProviderNoServerSide(t *testing.T) {
	svc := NewService()
	set := ports.UISettings{CaptchaEnabled: true, CaptchaProvider: ProviderTurnstile}
	ch, err := svc.Issue(context.Background(), set)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if ch != nil {
		t.Fatalf("token providers render client-side; Issue must return nil, got %+v", ch)
	}
}

func TestVerify_TokenProvider(t *testing.T) {
	var gotSecret, gotResp, gotIP string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotSecret = r.PostFormValue("secret")
		gotResp = r.PostFormValue("response")
		gotIP = r.PostFormValue("remoteip")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	svc := NewService()
	svc.endpoints[ProviderTurnstile] = srv.URL
	set := ports.UISettings{CaptchaEnabled: true, CaptchaProvider: ProviderTurnstile, CaptchaSecretKey: "sk_test"}

	ok, err := svc.Verify(context.Background(), set, Response{Token: "tok_abc", RemoteIP: "9.9.9.9"})
	if err != nil || !ok {
		t.Fatalf("turnstile success must verify: ok=%v err=%v", ok, err)
	}
	if gotSecret != "sk_test" || gotResp != "tok_abc" || gotIP != "9.9.9.9" {
		t.Fatalf("siteverify form wrong: secret=%q response=%q remoteip=%q", gotSecret, gotResp, gotIP)
	}
}

func TestVerify_TokenProviderRejects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
	}))
	defer srv.Close()
	svc := NewService()
	svc.endpoints[ProviderHCaptcha] = srv.URL
	set := ports.UISettings{CaptchaEnabled: true, CaptchaProvider: ProviderHCaptcha, CaptchaSecretKey: "sk"}
	if ok, _ := svc.Verify(context.Background(), set, Response{Token: "bad"}); ok {
		t.Fatal("success:false must not verify")
	}
}

func TestVerify_TokenMissingSecretFailsClosed(t *testing.T) {
	svc := NewService()
	set := ports.UISettings{CaptchaEnabled: true, CaptchaProvider: ProviderRecaptcha, CaptchaSecretKey: ""}
	ok, err := svc.Verify(context.Background(), set, Response{Token: "tok"})
	if ok || err == nil {
		t.Fatalf("missing secret must fail closed with an error: ok=%v err=%v", ok, err)
	}
}

func TestVerify_UnknownProvider(t *testing.T) {
	svc := NewService()
	set := ports.UISettings{CaptchaEnabled: true, CaptchaProvider: "bogus"}
	if ok, err := svc.Verify(context.Background(), set, Response{Token: "x"}); ok || err == nil {
		t.Fatalf("unknown provider must error: ok=%v err=%v", ok, err)
	}
}
