package notify

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	paymentv1 "payment-api/api/payment/v1"
	"payment-api/internal/models"
	"payment-api/internal/pkg/helpers"
	"payment-api/internal/pkg/idgen"
	"payment-api/internal/pkg/pay/wechatpay"
	"time"

	"payment-api/internal/repo"

	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
)

type WechatNotifier struct {
	request     *paymentv1.WxPayNotifyReq
	mysql       *repo.PayMysql
	config      models.WeChatPayConfig
	transaction payments.Transaction
	order       *models.PayOrder

	merchant models.PayMerchant
}

var _ Notifier = (*WechatNotifier)(nil)

func NewWechatNotifier(req *paymentv1.WxPayNotifyReq) *WechatNotifier {
	return &WechatNotifier{
		request: req,
		mysql:   repo.NewPayMysql(models.PayDb()),
	}
}

func (n *WechatNotifier) LoadConfig(ctx context.Context) error {
	merchantMysql := repo.NewMerchantMysql(models.PayDb())
	n.merchant = merchantMysql.OneMerchant(repo.NewReqOneMerchant(n.request.MerchantAccount))
	if n.merchant.ID == 0 {
		return errors.New("merchant not found")
	}
	channel := merchantMysql.OneMerchantChannel(repo.NewReqOneMerchantChannel(n.merchant.ID, "wechat"))
	if channel.ID == 0 {
		return errors.New("merchant channel not found")
	}
	n.config = channel.WeChatPayConfig()
	return nil
}

func (n *WechatNotifier) VerifyAndDecrypt(ctx context.Context) error {
	request := &http.Request{
		Header: make(http.Header, len(n.request.Headers)),
		Body:   io.NopCloser(bytes.NewReader(n.request.Body)),
	}
	for name, value := range n.request.Headers {
		request.Header.Set(name, value)
	}
	var result *wechatpay.NotifyResult
	notifyHandler, err := wechatpay.NewNotifyParser(n.config)
	if err != nil {
		return err
	}
	err = notifyHandler.Parse(ctx, request, &result)
	if err != nil {
		return err
	}
	n.transaction = result.Transaction
	return nil
}

func (n *WechatNotifier) ValidateOrder(ctx context.Context) error {
	t := &n.transaction
	if t.TradeState == nil || *t.TradeState != models.PayStatusSuccess {
		return errors.New("微信回调不是支付成功状态")
	}
	if t.OutTradeNo == nil || *t.OutTradeNo == "" || t.TransactionId == nil || *t.TransactionId == "" ||
		t.Mchid == nil || t.Appid == nil || t.Amount == nil || t.Amount.Total == nil ||
		t.Amount.PayerTotal == nil || t.Amount.Currency == nil {
		return errors.New("微信回调缺少必要的订单字段")
	}
	return nil
}

func (n *WechatNotifier) UpdateOrder(ctx context.Context) error {
	// 保存交易流水
	_ = (&saveTransaction{Transaction: n.transaction, Account: n.request.MerchantAccount}).Do(ctx)
	err := (&saveOrder{}).Do(ctx)
	return err
}

func (n *WechatNotifier) NotifyBusiness(ctx context.Context) error {

	return nil
}

type saveOrder struct {
}

func (s *saveOrder) Do(ctx context.Context) error {
	return nil
}

// 保存交易流水
type saveTransaction struct {
	Transaction payments.Transaction
	Account     string

	flowNo string
	mysql  *repo.MysqlFLow
}

func (s *saveTransaction) IsDuplicate() bool {
	s.mysql = repo.NewMysqlFlow(models.PayDb())
	flow := s.mysql.OneFlow(repo.NewReqOneFlow(*s.Transaction.TransactionId))
	return flow.ID > 0
}
func (s *saveTransaction) GenFlowNo() {
	widener, err := idgen.New(idgen.NodeID())
	if err != nil {
		return
	}
	s.flowNo = widener.Next()
	return
}

func (s *saveTransaction) Sync(ctx context.Context) error {
	t := s.Transaction
	tradeTime := time.Now()
	parseTime, err := helpers.ParseTime(*t.SuccessTime)
	if err == nil {
		tradeTime = parseTime
	}
	return s.mysql.ToFlow(&models.PayFlow{
		FlowNo:          s.flowNo,
		PayNo:           *t.OutTradeNo,              // 支付单号
		Channel:         models.ChannelCodeWechat,   // 支付渠道
		MerchantNo:      *t.Mchid,                   // 商户号
		AppID:           *t.Appid,                   // 应用ID
		OpenID:          *t.Payer.Openid,            // 用户标识
		AccountType:     *t.BankType,                // 账户类型
		TradeNo:         *t.TransactionId,           // 交易金额
		FlowType:        models.FlowTypePayment,     // 流水类型
		Direction:       models.FlowDirectionIncome, // 流向
		TotalAmount:     *t.Amount.Total,            // 交易金额
		RealAmount:      *t.Amount.PayerTotal,       // 实际金额
		Currency:        *t.Amount.PayerCurrency,    // 币种
		AccountedTime:   time.Now(),                 // 记账时间
		TradeTime:       tradeTime,                  // 交易时间
		Remark:          *t.Attach,                  // 备注
		MerchantAccount: s.Account,
	})
}

func (s *saveTransaction) Do(ctx context.Context) error {
	if s.IsDuplicate() {
		return nil
	}
	s.GenFlowNo()
	return s.Sync(ctx)
}
