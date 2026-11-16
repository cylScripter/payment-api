package models

import (
	"database/sql"
	"time"
)

const (
	ChannelCodeWechat = "wechat"
	ChannelCodeDouyin = "douyin"
	ChannelCodeAlipay = "alipay"
)

const (
	PayStatusPending = "PENDING" // 待支付
	PayStatusSuccess = "SUCCESS" // 支付成功
	PayStatusClosed  = "CLOSED"  // 已确认关闭
	PayStatusFailed  = "FAILED"
)

const (
	BusinessNotifyPending = "PENDING"
	BusinessNotifySuccess = "SUCCESS"
)

// PayOrder 支付单
type PayOrder struct {
	ID                   int64        `json:"id" gorm:"column:id;primary_key"`
	PayNo                string       `json:"pay_no" gorm:"column:pay_no;unique;not null"`                                     // 支付单号
	BizOrderNo           string       `json:"biz_order_no" gorm:"column:biz_order_no;"`                                        // 订单号
	BizID                string       `json:"biz_id" gorm:"column:biz_id;"`                                                    // 业务标识
	BizType              string       `json:"biz_type" gorm:"column:biz_type;"`                                                // 业务类型
	TotalAmount          int64        `json:"total_amount" gorm:"column:total_amount;"`                                        // 订单金额，单位分
	PayAmount            int64        `json:"pay_amount" gorm:"column:pay_amount;"`                                            // 用户实付金额，单位分
	RefundedAmount       int64        `json:"refunded_amount" gorm:"column:refunded_amount;not null;default:0"`                // 累计退款成功金额，单位分，与 TotalAmount 同口径
	Status               string       `json:"status" gorm:"column:status;not null;default:PENDING"`                            // 支付状态，退款后保持 SUCCESS
	BusinessNotifyStatus string       `json:"business_notify_status" gorm:"column:business_notify_status;not null;default:''"` // 业务通知状态，空值表示尚未收到支付回调
	Channel              string       `json:"channel" gorm:"column:channel;"`                                                  // 支付渠道
	TradeNo              string       `json:"trade_no" gorm:"column:trade_no;"`                                                // 渠道交易号
	Currency             string       `json:"currency" gorm:"column:currency;default:'CNY'"`                                   // 货币类型
	ClientIP             string       `json:"client_ip" gorm:"column:client_ip;"`                                              // 客户端IP
	ExpireTime           time.Time    `json:"expire_time" gorm:"column:expire_time;"`                                          // 订单过期时间
	NotifyURL            string       `json:"notify_url" gorm:"column:notify_url;"`                                            // 通知地址
	RawRequest           string       `json:"raw_request" gorm:"column:raw_request;"`                                          // 原始请求
	RawResponse          string       `json:"raw_response" gorm:"column:raw_response;"`                                        // 原始响应
	PayTime              sql.NullTime `json:"pay_time" gorm:"column:pay_time;"`                                                // 支付时间
	MerchantAccount      string       `json:"merchant_account" gorm:"column:merchant_account;"`                                // 账户
	MerchantNo           string       `json:"merchant_no" gorm:"column:merchant_no;"`                                          // 商户订单号
	AppID                string       `json:"app_id" gorm:"column:app_id;"`                                                    // 应用ID
	OpenID               string       `json:"open_id" gorm:"column:open_id;"`                                                  // 用户标识
	Version              int32        `json:"version" gorm:"column:version;"`                                                  // 版本号
	Subject              string       `json:"subject" gorm:"column:subject;"`                                                  // 商品描述
	CreateTime           time.Time    `json:"create_time" gorm:"column:create_time;not null;autoCreateTime"`                   // 创建时间                                            // 本地入库时间
	UpdateTime           time.Time    `json:"update_time" gorm:"column:update_time;not null;autoUpdateTime"`                   // 更新时间
}

func (PayOrder) TableName() string { return "pay_order" }
