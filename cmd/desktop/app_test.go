package main

import (
	"os"
	"reflect"
	"testing"
)

func TestAppCredentialBridgeIsScopedAndIdempotent(t *testing.T) {
	app := NewApp()
	app.SetCredential("node", "secret")
	if got := app.GetCredential("node"); got != "secret" {
		t.Fatalf("GetCredential() = %q, want secret", got)
	}
	app.DeleteCredential("node")
	if got := app.GetCredential("node"); got != "" {
		t.Fatalf("GetCredential() after delete = %q, want empty", got)
	}
	app.DeleteCredential("node")
	app.SetCredential("", "ignored")
	if got := app.GetCredential(""); got != "" {
		t.Fatalf("blank key should not be stored: %q", got)
	}
}

func TestAppPaseoDeploymentBridge(t *testing.T) {
	t.Setenv("PASEO_URL", " https://paseo.example.test/ ")
	t.Setenv("PASEO_ALLOWED_ORIGINS", " https://paseo.example.test, ,https://app.example.test ")
	app := NewApp()
	if got := app.GetPaseoURL(); got != "https://paseo.example.test/" {
		t.Fatalf("GetPaseoURL() = %q", got)
	}
	if got, want := app.GetPaseoAllowedOrigins(), []string{"https://paseo.example.test", "https://app.example.test"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("GetPaseoAllowedOrigins() = %#v, want %#v", got, want)
	}
	_ = os.Unsetenv("PASEO_URL")
	_ = os.Unsetenv("PASEO_ALLOWED_ORIGINS")
	if app.GetPaseoURL() != "" || len(app.GetPaseoAllowedOrigins()) != 0 {
		t.Fatal("empty deployment configuration must fail closed")
	}
}
