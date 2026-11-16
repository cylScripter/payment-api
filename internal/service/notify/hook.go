package notify

import (
	"context"
	"encoding/json"
	"payment-api/internal/models"
	"payment-api/internal/repo"
)

type LogHook struct {
	ReqId         string
	TradeNo       string
	PayNo         string
	Channel       string
	NotifyType    string
	RequestBody   interface{}
	VerifyStatus  string
	ProcessStatus string
	ErrMessage    string
}

func (l *LogHook) Do(ctx context.Context) error {
	body, _ := json.Marshal(l.RequestBody)
	mysql := repo.NewMySQLNotifyLog(models.PayDb())
	return mysql.Create(ctx, &models.PayNotifyLog{
		RequestId:     l.ReqId,
		TradeNo:       l.TradeNo,
		PayNo:         l.PayNo,
		Channel:       l.Channel,
		NotifyType:    l.NotifyType,
		RequestBody:   string(body),
		VerifyStatus:  l.VerifyStatus,
		ProcessStatus: l.ProcessStatus,
		ErrMessage:    l.ErrMessage,
	})
}
