package models

import "time"

const (
	NotifyTypePay    = "PAY"    // 支付通知
	NotifyTypeRefund = "REFUND" // 退款通知
)

type PayNotifyLog struct {
	ID            int64     `json:"id" gorm:"column:id;primary_key"`
	PayNo         string    `json:"pay_no" gorm:"column:pay_no;not null"`                          // 支付单号
	TradeNo       string    `json:"trade_no" gorm:"column:trade_no;not null"`                      // 渠道交易号
	Channel       string    `json:"channel" gorm:"column:channel;not null"`                        // 支付渠道
	NotifyType    string    `json:"notify_type" gorm:"column:notify_type;not null"`                // 通知类型
	RequestBody   string    `json:"request_body" gorm:"column:request_body;not null"`              // 请求内容
	VerifyStatus  string    `json:"verify_status" gorm:"column:verify_status;not null"`            // 验证状态
	ProcessStatus string    `json:"process_status" gorm:"column:process_status;not null"`          // 处理状态
	ErrMessage    string    `json:"err_message" gorm:"column:err_message;"`                        // 错误信息
	CreateTime    time.Time `json:"create_time" gorm:"column:create_time;not null;autoCreateTime"` // 本地入库时间
	UpdateTime    time.Time `json:"update_time" gorm:"column:update_time;not null;autoUpdateTime"`
	RequestId     string    `json:"request_id" gorm:"column:request_id;not null"` // 请求ID
}

func (PayNotifyLog) TableName() string { return "pay_notify_log" }
