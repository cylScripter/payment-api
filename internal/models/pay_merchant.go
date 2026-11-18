package models

import (
	"encoding/json"
	"payment-api/internal/config"
	"payment-api/internal/pkg/helpers"
	"time"
)

type PayMerchant struct {
	ID           int64     `json:"id" gorm:"column:id;primary_key"`
	CreateTime   time.Time `json:"create_time" gorm:"column:create_time;not null;autoCreateTime"` // 本地入库时间
	UpdateTime   time.Time `json:"update_time" gorm:"column:update_time;not null;autoUpdateTime"`
	Name         string    `json:"name" gorm:"column:name;not null"`                                    // 商户名称
	Account      string    `json:"account" gorm:"column:account;not null"`                              // 商户账号
	Type         int64     `json:"type" gorm:"column:type;not null"`                                    // 商户类型：1业务商户 2平台商户
	Status       int64     `json:"status" gorm:"column:status;not null"`                                // 状态：1正常 2禁用
	NotifyURL    string    `json:"notify_url" gorm:"column:notify_url;not null"`                        // 通知地址
	NotifySecret string    `json:"-" gorm:"column:notify_secret;type:varchar(512);not null;default:''"` // 回调密钥密文
	ReturnUrl    string    `json:"return_url" gorm:"column:return_url;not null"`                        // 回调地址
	Remark       string    `json:"remark" gorm:"column:remark;"`                                        // 备注
}

func (PayMerchant) TableName() string { return "pay_merchant" }

type PayMerchantChannel struct {
	ID                int64     `json:"id" gorm:"column:id;primary_key"`
	CreateTime        time.Time `json:"create_time" gorm:"column:create_time;not null;autoCreateTime"`  // 本地入库时间
	UpdateTime        time.Time `json:"update_time" gorm:"column:update_time;not null;autoUpdateTime"`  // 本地入库时间
	MerchantID        int64     `json:"merchant_id" gorm:"column:merchant_id;not null"`                 // 商户id
	ChannelCode       string    `json:"channel_code" gorm:"column:channel_code;not null"`               // 渠道编码 支付渠道：wechat/alipay
	ChannelMerchantNo string    `json:"channel_merchant_no" gorm:"column:channel_merchant_no;not null"` // 渠道商户号
	ChannelAppID      string    `json:"channel_app_id" gorm:"column:channel_app_id;not null"`           // 渠道appid
	AppSecret         string    `json:"app_secret" gorm:"column:app_secret;not null"`
	Config            string    `json:"config" gorm:"column:config;not null"` // 配置信息
	Status            int64     `json:"status" gorm:"column:status;not null"` // 状态：1正常 2禁用
}

func (p PayMerchantChannel) TableName() string { return "pay_merchant_channel" }

type WeChatPayConfig struct {
	AppID        string            `json:"app_id"`
	AppIDs       map[string]string `json:"app_ids,omitempty"` // 按 JSAPI、NATIVE、H5、APP 分别配置，未配置时使用 app_id。
	MchId        string            `json:"mch_id"`
	CertSerialNo string            `json:"cert_serial_no"`
	PrivateKey   string            `json:"private_key"`
	PublicKeyId  string            `json:"public_key_id"`
	PublicKey    string            `json:"public_key"`
	ApiV3Key     string            `json:"api_v3_key"`
	NotifyUrl    string            `json:"notify_url"`
}

func (p PayMerchantChannel) WeChatPayConfig() WeChatPayConfig {
	var doc WeChatPayConfig
	_ = json.Unmarshal([]byte(p.Config), &doc)
	var (
		privateKey string
		publicKey  string
		apiV3Key   string
	)
	privateKey, _ = helpers.DecryptPEMString(doc.PrivateKey, []byte(config.Get().EncryptKey))
	publicKey, _ = helpers.DecryptPEMString(doc.PublicKey, []byte(config.Get().EncryptKey))
	apiV3Key, _ = helpers.DecryptPEMString(doc.ApiV3Key, []byte(config.Get().EncryptKey))
	doc.PrivateKey = privateKey
	doc.PublicKey = publicKey
	doc.ApiV3Key = apiV3Key
	return doc
}
