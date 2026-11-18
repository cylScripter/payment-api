package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"payment-api/internal/config"
	"payment-api/internal/initials"
	"payment-api/internal/pkg/gormhelper"
	"payment-api/internal/service/merchant"
	"strings"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	account := flag.String("account", "", "商户账号")
	flag.Parse()
	if strings.TrimSpace(*account) == "" {
		return errors.New("-account 必填")
	}
	if err := config.Init(); err != nil {
		return err
	}
	if err := initials.InitDatabase(config.Get()); err != nil {
		return err
	}
	defer func() { _ = gormhelper.ClientInstance.Close() }()
	secret, err := merchant.NotifySecret(context.Background(), *account)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		MerchantAccount string `json:"merchant_account"`
		NotifySecret    string `json:"notify_secret"`
	}{
		MerchantAccount: *account,
		NotifySecret:    secret,
	})
}
