package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

// wechatCode2SessionClient is the subset of wechatMPClient the silent-login
// channel depends on. Declared as an interface so tests can stub code2session
// without a WeChat round trip.
type wechatCode2SessionClient interface {
	Code2Session(ctx context.Context, code string) (string, error)
}

// wechatAuthClients bundles the two remote deps of the silent-login channel.
// It is nil in normal operation and lazily built from config; tests assign
// it directly (same package).
type wechatAuthClients struct {
	wechat  wechatCode2SessionClient
	casdoor casdoorClient
}

// wechatAccountIdentifiers derives the deterministic local identity for a
// WeChat openid: a username capped at 8 chars of openid, and a reserved
// @wechat.local email that cannot collide with real addresses.
func wechatAccountIdentifiers(openid string) (username, email string) {
	suffix := openid
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	return "wx_" + suffix, "wx_" + openid + "@wechat.local"
}

// encryptWeChatServiceSecret seals plaintext with AES-256-GCM under
// sha256(key) and returns base64(nonce || ciphertext).
func encryptWeChatServiceSecret(key, plaintext string) (string, error) {
	gcm, err := wechatGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil)), nil
}

// decryptWeChatServiceSecret opens a ciphertext produced by
// encryptWeChatServiceSecret. GCM auth guarantees a wrong key or tampered
// ciphertext fails instead of returning garbage.
func decryptWeChatServiceSecret(key, ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	gcm, err := wechatGCM(key)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}

func wechatGCM(key string) (cipher.AEAD, error) {
	k := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(k[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// LoginWithWeChatCode implements the mini-program silent-login channel:
// wx.login code -> code2session openid -> (first login) provision Casdoor
// user with a random service password -> ROPC against Casdoor proves the
// account is still valid -> issue local tokens. The service password is
// persisted only as AES-GCM ciphertext under WECHAT_MP_SECRET_KEY; a stale
// one (admin reset, key rotation) is detected by ROPC failure and repaired
// by a reset + retry, mirroring the web flow's oidc-token -> userinfo path
// for first-login provisioning.
func (s *userService) LoginWithWeChatCode(
	ctx context.Context,
	code string,
	provisioning types.TenantProvisioningMode,
) (*types.LoginResponse, error) {
	clients, err := s.wechatClients()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("code is required")
	}
	openid, err := clients.wechat.Code2Session(ctx, code)
	if err != nil {
		return nil, err
	}
	username, email := wechatAccountIdentifiers(openid)
	secretKey := s.config.WechatMP.SecretKey

	user, lookupErr := s.userRepo.GetUserByEmail(ctx, email)
	notFound := isUserLookupNotFound(lookupErr) || user == nil
	if lookupErr != nil && !notFound {
		return nil, fmt.Errorf("failed to query user by email: %w", lookupErr)
	}

	isNewUser := false
	var svcPwd string
	if notFound {
		isNewUser = true
		svcPwd, err = generateRandomString(48)
		if err != nil {
			return nil, fmt.Errorf("failed to generate service password: %w", err)
		}
		if err := clients.casdoor.EnsureUser(ctx, username, email, svcPwd); err != nil {
			return nil, fmt.Errorf("casdoor provisioning failed: %w", err)
		}
	} else {
		svcPwd, err = decryptWeChatServiceSecret(secretKey, user.Preferences.WeChatServiceSecret)
		if err != nil {
			// 密文损坏(或密钥轮换后无法解密)视同服务密码失效,走重置兜底
			svcPwd = ""
		}
	}

	casdoorToken, ropcErr := clients.casdoor.PasswordToken(ctx, username, svcPwd)
	if ropcErr != nil {
		if isNewUser {
			return nil, fmt.Errorf("casdoor login failed after provisioning: %w", ropcErr)
		}
		// ROPC 失败兜底:本地保存的服务密码在 Casdoor 侧已失效,重置后重试一次,
		// 成功则回写新密文,下次静默登录恢复正常路径。
		svcPwd, err = generateRandomString(48)
		if err != nil {
			return nil, fmt.Errorf("failed to generate service password: %w", err)
		}
		if err := clients.casdoor.SetPassword(ctx, username, svcPwd); err != nil {
			return nil, fmt.Errorf("casdoor password reset failed: %w", err)
		}
		casdoorToken, ropcErr = clients.casdoor.PasswordToken(ctx, username, svcPwd)
		if ropcErr != nil {
			return nil, fmt.Errorf("casdoor login failed after reset: %w", ropcErr)
		}
		if user, lookupErr = s.userRepo.GetUserByEmail(ctx, email); lookupErr != nil || user == nil {
			return nil, fmt.Errorf("user disappeared during wechat login: %v", lookupErr)
		}
		if encrypted, encErr := encryptWeChatServiceSecret(secretKey, svcPwd); encErr == nil {
			user.Preferences.WeChatServiceSecret = encrypted
			if err := s.userRepo.UpdateUser(ctx, user); err != nil {
				return nil, fmt.Errorf("failed to persist wechat binding: %w", err)
			}
		}
	}

	if isNewUser {
		// 与 loginWithOIDC 一致:用 Casdoor token 换 userinfo 建档。WeChat 渠道
		// 不返回 id_token,claims 全部来自 userinfo 端点。
		cfg, err := s.getOIDCConfig(ctx)
		if err != nil {
			return nil, err
		}
		userInfo, err := s.resolveOIDCUserInfo(ctx, cfg, &oidcTokenResponse{AccessToken: casdoorToken})
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(userInfo.Email) == "" {
			userInfo.Email = email
		}
		user, err = s.provisionOIDCUser(ctx, userInfo, provisioning)
		if err != nil {
			return nil, err
		}
		if encrypted, encErr := encryptWeChatServiceSecret(secretKey, svcPwd); encErr == nil {
			user.Preferences.WeChatServiceSecret = encrypted
		}
		if err := s.userRepo.UpdateUser(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to persist wechat binding: %w", err)
		}
	}

	cb, err := s.finalizeLoginSession(ctx, user, isNewUser)
	if err != nil {
		return nil, err
	}
	return toLoginResponse(cb), nil
}

// wechatClients lazily builds the channel's remote clients from config.
func (s *userService) wechatClients() (*wechatAuthClients, error) {
	if s.wechat != nil {
		return s.wechat, nil
	}
	if s.config == nil || s.config.WechatMP == nil || s.config.CasdoorAdmin == nil ||
		s.config.OIDCAuth == nil {
		return nil, errors.New("wechat login is not configured")
	}
	s.wechat = &wechatAuthClients{
		wechat:  newWechatMPClient(s.config.WechatMP),
		casdoor: newCasdoorClient(s.config.CasdoorAdmin, s.config.OIDCAuth.ClientID, s.config.OIDCAuth.ClientSecret),
	}
	return s.wechat, nil
}

// toLoginResponse re-shapes the shared OIDC callback payload into the
// LoginResponse contract the mini-program client expects.
func toLoginResponse(r *types.OIDCCallbackResponse) *types.LoginResponse {
	return &types.LoginResponse{
		Success:      r.Success,
		Message:      r.Message,
		User:         r.User,
		ActiveTenant: r.Tenant,
		Memberships:  r.Memberships,
		Token:        r.Token,
		RefreshToken: r.RefreshToken,
	}
}
