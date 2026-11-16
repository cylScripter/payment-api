package models

import "time"

const (
	FlowDirectionIncome  = "INCOME"  // 收入
	FlowDirectionExpense = "EXPENSE" // 支出
)

const (
	FlowTypePayment = "PAYMENT" // 收款
	FlowTypeRefund  = "REFUND"  // 退款
	FlowTypeOther   = "OTHER"   // 其他资金变动，具体类型保存在 ChannelBizType
)

// PayFlow 记录渠道账户已确认的资金变动，账单导入时按渠道流水幂等入库。
type PayFlow struct {
	ID              int64     `json:"id" gorm:"column:id;"`
	FlowNo          string    `json:"flow_no" gorm:"column:flow_no;"`                                // 网关资金流水号
	PayNo           string    `json:"pay_no" gorm:"column:pay_no;"`                                  // 关联支付单号，可空
	RefundNo        string    `json:"refund_no" gorm:"column:refund_no;"`                            // 关联退款单号，可空
	MerchantAccount string    `json:"merchant_account" gorm:"column:merchant_account;"`              // 账户
	Channel         string    `json:"channel" gorm:"column:channel;"`                                // 支付渠道
	MerchantNo      string    `json:"merchant_no" gorm:"column:merchant_no;"`                        // 渠道商户号
	AppID           string    `json:"app_id" gorm:"column:app_id;"`                                  // 渠道应用ID
	OpenID          string    `json:"open_id" gorm:"column:open_id;"`                                // 用户在渠道的标识
	AccountType     string    `json:"account_type" gorm:"column:account_type;"`                      // 渠道资金账户类型，微信为 BASIC、OPERATION、FEES
	TradeNo         string    `json:"trade_no" gorm:"column:trade_no;"`                              // 渠道流水号
	RefundTradeNo   string    `json:"refund_trade_no" gorm:"column:refund_trade_no;"`                // 渠道退款流水号
	FlowType        string    `json:"flow_type" gorm:"column:flow_type;"`                            // 资金变动类型
	Direction       string    `json:"direction" gorm:"column:direction;"`                            // 收支方向
	TotalAmount     int64     `json:"total_amount" gorm:"column:total_amount;"`                      // 本次发生金额，正整数，单位分
	RealAmount      int64     `json:"real_amount" gorm:"column:real_amount;"`                        // 实际收/付款金额，正整数，单位分
	Currency        string    `json:"currency" gorm:"column:currency;"`                              // 币种
	AccountedTime   time.Time `json:"accounted_time" gorm:"column:accounted_time;"`                  // 收付款成功时间
	TradeTime       time.Time `json:"trade_time" gorm:"column:trade_time;"`                          // 账单日期
	Remark          string    `json:"remark" gorm:"column:remark;"`                                  // 账单备注
	CreateTime      time.Time `json:"create_time" gorm:"column:create_time;not null;autoCreateTime"` // 创建时间                                            // 本地入库时间
	UpdateTime      time.Time `json:"update_time" gorm:"column:update_time;not null;autoUpdateTime"` // 更新时间
}

func (PayFlow) TableName() string { return "pay_flow" }
