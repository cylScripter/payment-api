package initials

import (
	"payment-api/internal/config"
	"payment-api/internal/pkg/wechat"
)

func applyConfig(next config.Config) error {
	if err := wechat.Validate(next.Wechat); err != nil {
		return err
	}
	if err := InitDatabase(next.Database); err != nil {
		return err
	}
	return nil
}
