package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/Tencent/WeKnora/internal/config"
)

type wechatMPClient struct {
	appID      string
	appSecret  string
	baseURL    string
	httpClient *http.Client
}

func newWechatMPClient(cfg *config.WechatMPConfig) *wechatMPClient {
	return &wechatMPClient{
		appID:      cfg.AppID,
		appSecret:  cfg.AppSecret,
		baseURL:    "https://api.weixin.qq.com",
		httpClient: newOIDCHTTPClient(),
	}
}

// Code2Session exchanges a mini-program wx.login code for the user's openid
// via WeChat's sns/jscode2session endpoint.
func (w *wechatMPClient) Code2Session(ctx context.Context, code string) (string, error) {
	q := url.Values{
		"appid":      {w.appID},
		"secret":     {w.appSecret},
		"js_code":    {code},
		"grant_type": {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		w.baseURL+"/sns/jscode2session?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("wechat code2session request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var payload struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("wechat code2session: decode response: %w", err)
	}
	if payload.ErrCode != 0 || payload.OpenID == "" {
		return "", fmt.Errorf("wechat code2session: errcode=%d errmsg=%s", payload.ErrCode, payload.ErrMsg)
	}
	return payload.OpenID, nil
}
