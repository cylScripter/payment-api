package pay

import (
	"context"
	"errors"
	"fmt"
	"payment-api/internal/models"
	"payment-api/internal/pkg/helpers"
	"payment-api/internal/pkg/idgen"
	"payment-api/internal/repo"
	"payment-api/internal/service/merchant"
	"time"

	paymentv1 "payment-api/api/payment/v1"
)

type CreateOrder interface {
	LoadMerchant(ctx context.Context) error                        // 加载商户信息
	Validate() error                                               // 校验参数
	CheckIdempotency(ctx context.Context) (exists bool, err error) // 幂等性校验
	SavePayment(ctx context.Context) error                         // 保存支付信息
	SelectChannel(ctx context.Context) error                       // 选择支付渠道
	LoadConfig(ctx context.Context) error                          // 加载支付渠道配置
	BuildReq() error                                               // 构建请求
	ToPay(ctx context.Context) error                               // 发起支付
	GetPayResp() (*paymentv1.CreateOrderResp, error)               // 获取支付结果
}

type CreateOrderHandler struct {
	Order *Order

	account  string // 商户账号
	merchant models.PayMerchant
	payNo    string

	channels *ChannelRegistry
	payer    Payer
	mysql    *repo.PayMysql
}

func WithRequest(req *paymentv1.CreateOrderReq) func(c *CreateOrderHandler) {
	return func(c *CreateOrderHandler) {
		c.Order.Request = req
		c.account = req.Account
	}
}

func WithChannel(channel string) func(c *CreateOrderHandler) {
	return func(c *CreateOrderHandler) {
		c.Order.Channel = channel
	}
}

// Build 初始化本次下单状态并应用选项，每次请求使用独立的 handler。
func (c *CreateOrderHandler) Build(opts ...func(c *CreateOrderHandler)) *CreateOrderHandler {
	if c.Order == nil {
		c.Order = &Order{Request: &paymentv1.CreateOrderReq{}}
		c.mysql = repo.NewPayMysql(models.PayDb())
	}
	c.payer = nil
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *CreateOrderHandler) LoadMerchant(ctx context.Context) error {
	c.merchant = merchant.GetMerchant(c.account)
	if c.merchant.ID == 0 {
		return fmt.Errorf("商户不存在")
	}
	return nil
}

func (c *CreateOrderHandler) Validate() error {
	switch TradeType(c.Order.Request.TradeType) {
	case JSAPI, Native, H5, App:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedPayment, c.Order.Request.TradeType)
	}
}

func (c *CreateOrderHandler) CheckIdempotency(ctx context.Context) (exists bool, err error) {
	widener, err := idgen.New(idgen.NodeID())
	if err != nil {
		return false, errors.New("生成订单号失败")
	}
	c.payNo = widener.Next()
	c.Order.PayNo = c.payNo
	order, _ := c.mysql.OrderExists(c.payNo)
	if order != nil && order.ID > 0 {
		return true, nil
	}
	return false, nil
}

func (c *CreateOrderHandler) SelectChannel(ctx context.Context) error {
	if c.channels == nil {
		c.channels = defaultChannels
	}
	if c.Order.Channel == "" {
		c.Order.Channel = "wechat"
	}
	c.Order.ChannelConfig = merchant.GetChannel(int(c.merchant.ID), c.Order.Channel)
	if c.Order.ChannelConfig.ID == 0 {
		return errors.New("商户未开通该支付渠道")
	}
	c.Order.MerchantNo = c.Order.ChannelConfig.ChannelMerchantNo
	var err error
	c.payer, err = c.channels.NewPayer(c.Order)
	return err
}

func (c *CreateOrderHandler) LoadConfig(ctx context.Context) error {
	return c.payer.LoadConfig(ctx)
}

func (c *CreateOrderHandler) BuildReq() error {
	return c.payer.BuildReq()
}

func (c *CreateOrderHandler) SavePayment(ctx context.Context) error {
	order := c.Order
	req := order.Request
	payment := &models.PayOrder{
		PayNo:           order.PayNo,
		BizOrderNo:      req.OrderNo,
		Channel:         order.Channel,
		TotalAmount:     helpers.YuanToFen(req.Amount),
		Currency:        "CNY",
		ClientIP:        req.PayerClientIp,
		NotifyURL:       order.NotifyURL,
		RawRequest:      c.payer.GetPayReq(),
		MerchantAccount: c.account,
		MerchantNo:      order.MerchantNo,
		AppID:           order.AppID,
		Status:          models.PayStatusPending,
		Subject:         req.Description,
		Version:         1,
		ExpireTime:      time.Now(),
		OpenID:          req.Openid,
	}
	if err := c.mysql.ToOrder(ctx, payment); err != nil {
		return fmt.Errorf("保存订单失败: %w", err)
	}
	return nil
}

func (c *CreateOrderHandler) ToPay(ctx context.Context) error {
	var payErr error
	switch TradeType(c.Order.Request.TradeType) {
	case JSAPI:
		payErr = c.payer.JSAPI(ctx)
	case Native:
		payErr = c.payer.Native(ctx)
	case H5:
		payErr = c.payer.H5(ctx)
	case App:
		payErr = c.payer.App(ctx)
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedPayment, c.Order.Request.TradeType)
	}
	if payErr != nil {
		_ = c.mysql.UpdateOrder(c.payNo, map[string]interface{}{
			"status":       models.PayStatusFailed,
			"raw_response": c.payer.GetRawResp(),
		})
		return payErr
	}
	_ = c.mysql.UpdateOrder(c.payNo, map[string]interface{}{
		"raw_response": c.payer.GetRawResp(),
	})
	return nil
}

func (c *CreateOrderHandler) GetPayResp() (*paymentv1.CreateOrderResp, error) {
	if c.Order.Response != nil {
		return c.Order.Response, nil
	}
	if c.payer == nil {
		return nil, errors.New("已有支付订单缺少返回结果")
	}
	return c.payer.GetPayResp()
}

type PaymentHandler struct {
	handler CreateOrder
}

func NewPaymentHandler(handler CreateOrder) *PaymentHandler {
	return &PaymentHandler{handler: handler}
}

func (p PaymentHandler) CreateOrder(ctx context.Context) (*paymentv1.CreateOrderResp, error) {
	if err := p.handler.LoadMerchant(ctx); err != nil {
		return nil, err
	}
	if err := p.handler.Validate(); err != nil {
		return nil, err
	}
	ok, err := p.handler.CheckIdempotency(ctx)
	if err != nil {
		return nil, err
	}
	if ok {
		return p.handler.GetPayResp()
	}
	if err := p.handler.SelectChannel(ctx); err != nil {
		return nil, err
	}
	if err := p.handler.LoadConfig(ctx); err != nil {
		return nil, err
	}
	if err := p.handler.BuildReq(); err != nil {
		return nil, err
	}
	if err := p.handler.SavePayment(ctx); err != nil {
		return nil, err
	}
	if err := p.handler.ToPay(ctx); err != nil {
		return nil, err
	}
	return p.handler.GetPayResp()
}
