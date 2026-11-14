package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"payment-api/internal/config"
)

var ErrNotConfigured = errors.New("微信小程序登录配置尚未设置")

var client = &http.Client{
	Timeout:       10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
}

type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("微信小程序登录失败，错误码 %d，%s", e.Code, e.Message)
}

func Init() error { return Validate(config.Get().Wechat) }

func Validate(cfg config.Wechat) error {
	if cfg == (config.Wechat{}) {
		return nil
	}
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.AppSecret) == "" {
		return errors.New("wechat.app_id 和 wechat.app_secret 须填写完整")
	}

	return nil
}

func ExchangeCode(ctx context.Context, code string) (string, error) {
	cfg := config.Get().Wechat
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return "", ErrNotConfigured
	}
	query := url.Values{
		"appid":      {cfg.AppID},
		"secret":     {cfg.AppSecret},
		"js_code":    {code},
		"grant_type": {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.weixin.qq.com/sns/jscode2session?"+query.Encode(), nil)
	if err != nil {
		return "", errors.New("创建微信小程序登录请求失败")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("微信小程序登录请求失败，请重新调用 wx.login 获取 code")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("微信小程序登录接口返回 HTTP %d", resp.StatusCode)
	}
	var result struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return "", errors.New("读取微信小程序登录应答失败")
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", errors.New("微信小程序登录应答格式错误")
	}
	if result.ErrCode != 0 {
		message := strings.NewReplacer(cfg.AppSecret, "[redacted]", code, "[redacted]").Replace(result.ErrMsg)
		return "", &APIError{Code: result.ErrCode, Message: message}
	}
	if strings.TrimSpace(result.OpenID) == "" {
		return "", errors.New("微信小程序登录应答缺少 openid")
	}
	return result.OpenID, nil
}
