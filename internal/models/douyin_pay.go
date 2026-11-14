package models

import (
	"encoding/json"
	"fmt"

	"payment-api/internal/config"
	"payment-api/internal/pkg/helpers"
)

type DouyinPayConfig struct {
	AppID        string            `json:"app_id"`
	AppIDs       map[string]string `json:"app_ids,omitempty"`
	MchID        string            `json:"mch_id"`
	CertSerialNo string            `json:"cert_serial_no"`
	PrivateKey   string            `json:"private_key"`   // 沿用 helpers.EncryptPEMString 加密后的密文。
	PlatformCert string            `json:"platform_cert"` // 抖音支付平台证书 PEM。
	NotifyURL    string            `json:"notify_url"`
}

func (p PayMerchantChannel) DouyinPayConfig() (DouyinPayConfig, error) {
	var cfg DouyinPayConfig
	if err := json.Unmarshal([]byte(p.Config), &cfg); err != nil {
		return cfg, fmt.Errorf("抖音支付配置 JSON 格式错误: %w", err)
	}
	privateKey, err := helpers.DecryptPEMString(cfg.PrivateKey, []byte(config.Get().EncryptKey))
	if err != nil {
		return cfg, fmt.Errorf("解密抖音支付商户私钥失败: %w", err)
	}
	cfg.PrivateKey = privateKey
	return cfg, nil
}
