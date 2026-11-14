package models

import (
	"encoding/json"
	"fmt"

	"payment-api/internal/config"
	"payment-api/internal/pkg/helpers"
)

// AlipayConfig 使用公钥模式，配置保存在商户渠道的 config 字段。
type AlipayConfig struct {
	AppID           string `json:"app_id"`
	SellerID        string `json:"seller_id"`
	PrivateKey      string `json:"private_key"` // helpers.EncryptPEMString 生成的密文。
	AlipayPublicKey string `json:"alipay_public_key"`
	NotifyURL       string `json:"notify_url"`
	ReturnURL       string `json:"return_url,omitempty"`
	Sandbox         bool   `json:"sandbox"`
}

func (p PayMerchantChannel) AlipayConfig() (AlipayConfig, error) {
	var cfg AlipayConfig
	if err := json.Unmarshal([]byte(p.Config), &cfg); err != nil {
		return cfg, fmt.Errorf("支付宝配置 JSON 格式错误: %w", err)
	}
	privateKey, err := helpers.DecryptPEMString(cfg.PrivateKey, []byte(config.Get().EncryptKey))
	if err != nil {
		return cfg, fmt.Errorf("解密支付宝应用私钥失败: %w", err)
	}
	cfg.PrivateKey = privateKey

	alipayPublicKey, err := helpers.DecryptPEMString(cfg.AlipayPublicKey, []byte(config.Get().EncryptKey))
	if err != nil {
		return cfg, fmt.Errorf("解密支付宝公钥失败: %w", err)
	}
	cfg.AlipayPublicKey = alipayPublicKey
	return cfg, nil
}
