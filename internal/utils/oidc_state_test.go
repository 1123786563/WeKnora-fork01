package utils

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSignAndVerifyOIDCState(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-oidc-state-secret")
	oidcStateSecretOnce = sync.Once{}
	oidcStateSecret = ""

	state, err := SignOIDCState(&OIDCStatePayload{
		Nonce:       "nonce-abc",
		RedirectURI: "http://localhost:5173/login",
		IssuedAt:    time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("SignOIDCState: %v", err)
	}
	got, err := VerifyOIDCState(state)
	if err != nil {
		t.Fatalf("VerifyOIDCState: %v", err)
	}
	if got.Nonce != "nonce-abc" || got.RedirectURI != "http://localhost:5173/login" {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestVerifyOIDCStateRejectsTamperedPayload(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-oidc-state-secret")
	oidcStateSecretOnce = sync.Once{}
	oidcStateSecret = ""

	state, err := SignOIDCState(&OIDCStatePayload{
		Nonce:       "nonce-abc",
		RedirectURI: "http://localhost:5173/login",
		IssuedAt:    time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("SignOIDCState: %v", err)
	}
	parts := strings.Split(state, ".")
	tampered := parts[0] + ".AAAA"
	if _, err := VerifyOIDCState(tampered); err == nil {
		t.Fatal("expected tampered state to be rejected")
	}
}

func TestOIDCCodeChallengeUsesS256(t *testing.T) {
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	const challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

	got, err := OIDCCodeChallenge(verifier)
	if err != nil {
		t.Fatalf("OIDCCodeChallenge: %v", err)
	}
	if got != challenge {
		t.Fatalf("challenge = %q, want %q", got, challenge)
	}
	if err := VerifyOIDCCodeChallenge(verifier, challenge); err != nil {
		t.Fatalf("VerifyOIDCCodeChallenge: %v", err)
	}
	if err := VerifyOIDCCodeChallenge("wrong-verifier", challenge); err == nil {
		t.Fatal("wrong verifier was accepted")
	}
}
