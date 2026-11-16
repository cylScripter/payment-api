package notify

import (
	"context"
	"log/slog"
)

// Notifier 由各支付渠道实现，每次回调创建独立对象。
type Notifier interface {
	LoadConfig(ctx context.Context) error
	VerifyAndDecrypt(ctx context.Context) error
	ValidateOrder(ctx context.Context) error
	UpdateOrder(ctx context.Context) error // 幂等更新支付结果
	NotifyBusiness(ctx context.Context) error
}

func Handle(ctx context.Context, notifier Notifier) error {
	steps := []func(context.Context) error{
		notifier.LoadConfig,
		notifier.VerifyAndDecrypt,
		notifier.ValidateOrder,
		notifier.UpdateOrder,
	}
	for _, step := range steps {
		if err := step(ctx); err != nil {
			return err
		}
	}
	if err := notifier.NotifyBusiness(ctx); err != nil {
		slog.ErrorContext(ctx, "业务通知失败，等待后台重试", "error", err)
	}
	return nil
}
