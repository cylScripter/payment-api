package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"payment-api/internal/config"
	"payment-api/internal/models"
	"payment-api/internal/pkg/helpers"
	"strconv"
	"time"

	"payment-api/internal/repo"

	log "github.com/sirupsen/logrus"
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
	Timeout: 5 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func sendBusinessNotify(ctx context.Context, merchant models.PayMerchant, payload *BusinessNotify) error {
	if merchant.NotifySecret == "" {
		return errors.New("商户未配置回调密钥")
	}
	secret, err := helpers.DecryptPEMString(merchant.NotifySecret, []byte(config.Get().EncryptKey))
	if err != nil || secret == "" {
		return errors.New("解密商户回调密钥失败")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v1\n" + timestamp + "\n"))
	mac.Write(body)
	mac.Write([]byte("\n"))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, merchant.NotifyURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pay-Timestamp", timestamp)
	req.Header.Set("X-Pay-Signature", "v1="+hex.EncodeToString(mac.Sum(nil)))
	resp, err := businessClient.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Errorf("关闭响应体失败: %v", err)
		}
	}(resp.Body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("业务通知返回 HTTP %d", resp.StatusCode)
	}
	respBody, _ := io.ReadAll(resp.Body)
	log.Infof("[BusinessNotify] 通知业务系统成功,req: %v resp: %s", string(body), respBody)
	return nil
}

// StartBusinessNotify 定期投递支付单中的待通知记录，停止时等待任务退出。
func StartBusinessNotify() func() {

	return func() {

	}
}

func RetryBusinessNotifies(ctx context.Context, mysql *repo.PayMysql) {

}
