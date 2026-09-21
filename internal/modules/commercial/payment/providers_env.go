package payment

import (
	"fmt"
	"os"
	"strings"
)

// ProvidersFromEnv builds the configured payment channel adapters from
// environment KEY REFERENCES (file paths and identifiers, never embedded
// key material). It is the production construction path shared by the
// order pipeline and the payment callback endpoint.
//
// An environment with no channel variables at all yields an empty map:
// blocked-env stays legal, and checkout/callbacks then fail closed with
// explicit unconfigured errors. A PARTIALLY configured channel is a
// deployment error and returns a non-nil error so startup fails loudly
// instead of silently dropping a provider whose paid orders could then
// never be confirmed.
func ProvidersFromEnv() (map[string]Provider, error) {
	return providersFromEnv(os.Getenv)
}

func providersFromEnv(getenv func(string) string) (map[string]Provider, error) {
	providers := map[string]Provider{}

	alipayCfg, present, err := alipayConfigFromEnv(getenv)
	if err != nil {
		return nil, err
	}
	if present {
		p, err := NewAlipayProvider(alipayCfg)
		if err != nil {
			return nil, fmt.Errorf("alipay provider: %w", err)
		}
		providers[ProviderAlipay] = p
	}

	wechatCfg, present, err := wechatConfigFromEnv(getenv)
	if err != nil {
		return nil, err
	}
	if present {
		p, err := NewWechatProvider(wechatCfg)
		if err != nil {
			return nil, fmt.Errorf("wechat provider: %w", err)
		}
		providers[ProviderWechat] = p
	}
	return providers, nil
}

// alipayConfigFromEnv reads the WEKNORA_ALIPAY_* references. present=false
// means no Alipay variable is set at all; any subset of the required set
// without the whole set is an error.
func alipayConfigFromEnv(getenv func(string) string) (AlipayConfig, bool, error) {
	var cfg AlipayConfig
	set := map[string]*string{
		"WEKNORA_ALIPAY_APP_ID":            &cfg.AppID,
		"WEKNORA_ALIPAY_SELLER_ID":         &cfg.SellerID,
		"WEKNORA_ALIPAY_PUBLIC_KEY_PATH":   &cfg.AlipayPublicKeyPath,
		"WEKNORA_ALIPAY_MERCHANT_KEY_PATH": &cfg.MerchantPrivKeyPath,
	}
	present := false
	for name, dst := range set {
		v := strings.TrimSpace(getenv(name))
		if v == "" {
			continue
		}
		present = true
		*dst = v
	}
	if !present {
		return AlipayConfig{}, false, nil
	}
	for name, dst := range set {
		if *dst == "" {
			return AlipayConfig{}, false, fmt.Errorf(
				"alipay channel partially configured: WEKNORA_ALIPAY_%s missing; APP_ID, SELLER_ID, PUBLIC_KEY_PATH and MERCHANT_KEY_PATH are required together",
				strings.TrimPrefix(name, "WEKNORA_ALIPAY_"))
		}
	}
	// Optional references: gateway override, async notify URL and explicit
	// sign type. NotifyURL left empty means orders reconcile only through
	// the manual GetOrder recovery path — payments still verify, the
	// channel just has no push endpoint to call.
	cfg.GatewayURL = strings.TrimSpace(getenv("WEKNORA_ALIPAY_GATEWAY_URL"))
	cfg.NotifyURL = strings.TrimSpace(getenv("WEKNORA_ALIPAY_NOTIFY_URL"))
	cfg.SignType = strings.TrimSpace(getenv("WEKNORA_ALIPAY_SIGN_TYPE"))
	return cfg, true, nil
}

// wechatConfigFromEnv reads the WEKNORA_WECHAT_* references. WEKNORA_WECHAT_PLATFORM_CERTS
// is a comma-separated list of serial:public_key_path entries covering
// every platform certificate currently in rotation.
func wechatConfigFromEnv(getenv func(string) string) (WechatConfig, bool, error) {
	var cfg WechatConfig
	required := map[string]*string{
		"WEKNORA_WECHAT_APP_ID":         &cfg.AppID,
		"WEKNORA_WECHAT_MCH_ID":         &cfg.MchID,
		"WEKNORA_WECHAT_MCH_SERIAL":     &cfg.MchSerial,
		"WEKNORA_WECHAT_MCH_KEY_PATH":   &cfg.MchKeyPath,
		"WEKNORA_WECHAT_APIV3_KEY_PATH": &cfg.APIv3KeyPath,
	}
	present := false
	for name, dst := range required {
		v := strings.TrimSpace(getenv(name))
		if v == "" {
			continue
		}
		present = true
		*dst = v
	}
	if raw := strings.TrimSpace(getenv("WEKNORA_WECHAT_PLATFORM_CERTS")); raw != "" {
		present = true
		for _, entry := range strings.Split(raw, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			serial, path, ok := strings.Cut(entry, ":")
			serial, path = strings.TrimSpace(serial), strings.TrimSpace(path)
			if !ok || serial == "" || path == "" {
				return WechatConfig{}, false, fmt.Errorf(
					"WEKNORA_WECHAT_PLATFORM_CERTS entry %q must be serial:public_key_path", entry)
			}
			cfg.PlatformCerts = append(cfg.PlatformCerts, WechatPlatformCertRef{Serial: serial, PublicKeyPath: path})
		}
	}
	if !present {
		return WechatConfig{}, false, nil
	}
	for name, dst := range required {
		if *dst == "" {
			return WechatConfig{}, false, fmt.Errorf(
				"wechat channel partially configured: %s missing; APP_ID, MCH_ID, MCH_SERIAL, MCH_KEY_PATH, APIV3_KEY_PATH and PLATFORM_CERTS are required together", name)
		}
	}
	if len(cfg.PlatformCerts) == 0 {
		return WechatConfig{}, false, fmt.Errorf(
			"wechat channel partially configured: WEKNORA_WECHAT_PLATFORM_CERTS missing; APP_ID, MCH_ID, MCH_SERIAL, MCH_KEY_PATH, APIV3_KEY_PATH and PLATFORM_CERTS are required together")
	}
	cfg.NotifyURL = strings.TrimSpace(getenv("WEKNORA_WECHAT_NOTIFY_URL"))
	cfg.APIBaseURL = strings.TrimSpace(getenv("WEKNORA_WECHAT_API_BASE_URL"))
	return cfg, true, nil
}
