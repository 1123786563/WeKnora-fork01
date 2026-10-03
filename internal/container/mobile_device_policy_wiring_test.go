package container

import (
	"reflect"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler"
)

// CAREER-OCR H5: the registration handler must consume the same
// MOBILE_ENTERPRISE_APP_ID declaration the push routing side reads
// (container.go newMobileAppNotificationRouting). Without the policy wiring a
// declared enterprise app can be pushed to but never registers — the two
// halves of the enterprise mobile lane disagree.
func TestNewMobileDeviceHandlerAssemblesEnterprisePolicy(t *testing.T) {
	enterpriseAppID := func(h *handler.MobileDeviceHandler) string {
		policy := reflect.ValueOf(h).Elem().FieldByName("appPolicy")
		if !policy.IsValid() {
			t.Fatal("MobileDeviceHandler has no appPolicy field")
		}
		return policy.FieldByName("EnterpriseAppID").String()
	}

	t.Run("no declaration stays official-only (fail closed)", func(t *testing.T) {
		t.Setenv("MOBILE_ENTERPRISE_APP_ID", "")
		h := NewMobileDeviceHandler(nil)
		if got := enterpriseAppID(h); got != "" {
			t.Errorf("undeclared enterprise app must keep the zero policy, got %q", got)
		}
	})

	t.Run("valid declaration is wired into the policy", func(t *testing.T) {
		t.Setenv("MOBILE_ENTERPRISE_APP_ID", "enterprise:acme")
		h := NewMobileDeviceHandler(nil)
		if got := enterpriseAppID(h); got != "enterprise:acme" {
			t.Errorf("declared enterprise app must reach the policy, got %q", got)
		}
	})

	t.Run("invalid declaration fails closed to official-only", func(t *testing.T) {
		t.Setenv("MOBILE_ENTERPRISE_APP_ID", "not an app id")
		h := NewMobileDeviceHandler(nil)
		if got := enterpriseAppID(h); got != "" {
			t.Errorf("invalid enterprise app must be dropped (official-only), got %q", got)
		}
	})

	t.Run("official id as enterprise declaration is a no-op", func(t *testing.T) {
		t.Setenv("MOBILE_ENTERPRISE_APP_ID", repository.MobileAppIDOfficial)
		h := NewMobileDeviceHandler(nil)
		if got := enterpriseAppID(h); got != "" {
			t.Errorf("official id declared as enterprise must not widen the policy, got %q", got)
		}
	})
}
