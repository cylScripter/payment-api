package pay

import (
	"errors"
	"fmt"
	"maps"
)

var ErrUnsupportedPayment = errors.New("不支持的支付渠道或支付方式")

// ChannelFactory 为每次下单创建独立的渠道 Payer，可通过闭包共享 SDK 客户端。
type ChannelFactory func(order *Order) (Payer, error)

// ChannelRegistry 构造后只读，可由多个下单请求共享。
type ChannelRegistry struct {
	factories map[string]ChannelFactory
}

var defaultChannels = &ChannelRegistry{factories: map[string]ChannelFactory{
	"wechat": NewWechatPayer,
}}

func NewChannelRegistry(factories map[string]ChannelFactory) (*ChannelRegistry, error) {
	for name, factory := range factories {
		if name == "" {
			return nil, errors.New("支付渠道不能为空")
		}
		if factory == nil {
			return nil, fmt.Errorf("支付渠道工厂不能为空: %s", name)
		}
	}
	return &ChannelRegistry{factories: maps.Clone(factories)}, nil
}

func (r *ChannelRegistry) NewPayer(order *Order) (Payer, error) {
	factory, ok := r.factories[order.Channel]
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", ErrUnsupportedPayment, order.Channel, order.Request.TradeType)
	}
	payer, err := factory(order)
	if err != nil {
		return nil, err
	}
	if payer == nil {
		return nil, fmt.Errorf("渠道返回空支付策略: %s/%s", order.Channel, order.Request.TradeType)
	}
	return payer, nil
}
