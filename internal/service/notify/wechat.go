package notify

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
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

	sdknotify "github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
)

type WechatNotifier struct {
	request      *paymentv1.WxPayNotifyReq
	mysql        *repo.PayMysql
	config       models.WeChatPayConfig
	transaction  payments.Transaction
	order        *models.PayOrder
	verifyStatus string

	merchant models.PayMerchant
}

var _ Notifier = (*WechatNotifier)(nil)

func NewWechatNotifier(req *paymentv1.WxPayNotifyReq) *WechatNotifier {
	return &WechatNotifier{
		request:      req,
		mysql:        repo.NewPayMysql(models.PayDb()),
		verifyStatus: models.PayStatusPending,
	}
}

func (n *WechatNotifier) LoadConfig(ctx context.Context) error {
	merchantMysql := repo.NewMerchantMysql(models.PayDb())
	n.merchant, _ = merchantMysql.OneMerchant(ctx, repo.NewReqOneMerchant(n.request.MerchantAccount))
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
	n.verifyStatus = models.PayStatusFailed
	request := &http.Request{
		Header: make(http.Header, len(n.request.Headers)),
		Body:   io.NopCloser(bytes.NewReader(n.request.Body)),
	}
	for name, value := range n.request.Headers {
		request.Header.Set(name, value)
	}
	var result wechatpay.NotifyResult
	notifyHandler, err := wechatpay.NewNotifyParser(n.config)
	if err != nil {
		return err
	}
	err = notifyHandler.Parse(ctx, request, &result)
	if err != nil {
		return err
	}
	n.transaction = result.Transaction
	n.verifyStatus = models.PayStatusSuccess
	return nil
}

func (n *WechatNotifier) ValidateOrder(ctx context.Context) error {
	t := &n.transaction
	if t.TradeState == nil || *t.TradeState != models.PayStatusSuccess {
		return errors.New("微信回调不是支付成功状态")
	}
	if t.OutTradeNo == nil || *t.OutTradeNo == "" || t.TransactionId == nil || *t.TransactionId == "" ||
		t.Mchid == nil || *t.Mchid == "" || t.Appid == nil || *t.Appid == "" {
		return errors.New("微信回调缺少必要的订单字段")
	}
	if t.Payer == nil || t.Payer.Openid == nil || *t.Payer.Openid == "" ||
		t.BankType == nil || *t.BankType == "" {
		return errors.New("微信回调缺少付款人或银行类型")
	}
	if t.Amount == nil || t.Amount.Total == nil || t.Amount.PayerTotal == nil ||
		t.Amount.Currency == nil || t.Amount.PayerCurrency == nil {
		return errors.New("微信回调缺少金额或币种")
	}
	if *t.Amount.Total <= 0 || *t.Amount.PayerTotal < 0 || *t.Amount.PayerTotal > *t.Amount.Total {
		return errors.New("微信回调金额不合法")
	}
	if *t.Amount.Currency == "" || *t.Amount.PayerCurrency == "" {
		n.transaction.Amount.Currency = new("")
		n.transaction.Amount.PayerCurrency = new("")
	}
	if t.SuccessTime == nil {
		n.transaction.SuccessTime = new("")
	}
	if t.Attach == nil {
		t.Attach = new("")
	}
	return nil
}

func (n *WechatNotifier) UpdateOrder(ctx context.Context) error {
	// 保存交易流水
	_ = (&saveTransaction{Transaction: n.transaction, Account: n.request.MerchantAccount}).Do(ctx)
	err := (&saveOrder{
		Transaction: n.transaction,
		Account:     n.request.MerchantAccount,
	}).Do(ctx)
	return err
}

func (n *WechatNotifier) NotifyBusiness(ctx context.Context) error {
	order, err := n.mysql.OrderExists(*n.transaction.OutTradeNo)
	if err != nil {
		return err
	}
	if order.BusinessNotifyStatus == models.BusinessNotifySuccess {
		return nil
	}
	if err := n.mysql.UpdateNotifyStatus(ctx, order.PayNo, models.BusinessNotifyPending); err != nil {
		return err
	}
	payload := &BusinessNotify{
		OutTradeNo:      order.PayNo,
		OrderNo:         order.BizOrderNo,
		MerchantAccount: order.MerchantAccount,
		TradeNo:         order.TradeNo,
		Channel:         order.Channel,
		Status:          order.Status,
		TotalAmount:     order.TotalAmount,
		PayAmount:       order.PayAmount,
		PayTime:         order.PayTime.Time.Format(time.RFC3339),
		OpenId:          order.OpenID,
	}
	if n.transaction.TradeType != nil {
		payload.TradeType = *n.transaction.TradeType
	}
	if err := sendBusinessNotify(ctx, n.merchant, payload); err != nil {
		return err
	}
	return n.mysql.UpdateNotifyStatus(ctx, order.PayNo, models.BusinessNotifySuccess)
}

func (n *WechatNotifier) SaveLog(ctx context.Context, processErr error) error {
	var notification sdknotify.Request
	_ = json.Unmarshal(n.request.Body, &notification)
	hook := LogHook{
		ReqId:         notification.ID,
		Channel:       models.ChannelCodeWechat,
		NotifyType:    models.NotifyTypePay,
		RequestBody:   n.request,
		VerifyStatus:  n.verifyStatus,
		ProcessStatus: models.PayStatusSuccess,
	}
	if n.transaction.OutTradeNo != nil {
		hook.PayNo = *n.transaction.OutTradeNo
	}
	if n.transaction.TransactionId != nil {
		hook.TradeNo = *n.transaction.TransactionId
	}
	if processErr != nil {
		hook.ProcessStatus = models.PayStatusFailed
		hook.ErrMessage = processErr.Error()
	}
	return hook.Do(ctx)
}

type saveOrder struct {
	Transaction payments.Transaction
	Account     string
}

func (s *saveOrder) Do(ctx context.Context) error {
	t := s.Transaction
	paidAt, err := helpers.ParseTime(*t.SuccessTime)
	if err != nil {
		return err
	}
	mysql := repo.NewPayMysql(models.PayDb())
	return mysql.ConfirmOrder(ctx, &models.PayOrder{
		PayNo:           *t.OutTradeNo,
		MerchantAccount: s.Account,
		Channel:         models.ChannelCodeWechat,
		MerchantNo:      *t.Mchid,
		AppID:           *t.Appid,
		TradeNo:         *t.TransactionId,
		TotalAmount:     *t.Amount.Total,
		PayAmount:       *t.Amount.PayerTotal,
		Currency:        *t.Amount.Currency,
		OpenID:          *t.Payer.Openid,
		PayTime:         sql.NullTime{Time: paidAt, Valid: true},
	})
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
