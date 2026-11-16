package notify

import (
	"context"
	"net/http"
	"time"

	"payment-api/internal/models"
	"payment-api/internal/repo"
)

// BusinessNotify 金额均为整数分，业务方按 PayNo 幂等处理。
type BusinessNotify struct {
	OutTradeNo      string `json:"out_trade_no"`     // 商户订单号
	OrderNo         string `json:"order_no"`         // 业务方订单号
	MerchantAccount string `json:"merchant_account"` // 商户号
	TradeNo         string `json:"trade_no"`         // 微信交易单号
	Channel         string `json:"channel"`          // 微信支付 wechat
	Status          string `json:"status"`           // SUCCESS
	TotalAmount     int64  `json:"total_amount"`     // 订单总金额
	PayAmount       int64  `json:"pay_amount"`       // 实际支付金额
	PayTime         string `json:"pay_time"`         // 支付时间
	OpenId          string `json:"open_id"`          // 用户标识
	TradeType       string `json:"trade_type"`       // JSAPI、NATIVE、APP、MWEB、MINIPROG
}

var businessClient = &http.Client{
	Timeout: 2 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func sendBusinessNotify(ctx context.Context, order *models.PayOrder, mysql *repo.PayMysql) error {

	return nil
}

// StartBusinessNotify 定期投递支付单中的待通知记录，停止时等待任务退出。
func StartBusinessNotify() func() {

	return func() {

	}
}

func RetryBusinessNotifies(ctx context.Context, mysql *repo.PayMysql) {

}
