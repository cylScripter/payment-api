package merchant

import (
	"context"
	"errors"
	"payment-api/internal/config"
	"payment-api/internal/models"
	"payment-api/internal/pkg/helpers"
	"payment-api/internal/repo"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// NotifySecret 初始化并返回商户回调密钥，重复执行复用已保存的密钥。
func NotifySecret(ctx context.Context, account string) (string, error) {
	mysql := repo.NewMerchantMysql(models.PayDb())
	merchant, err := mysql.OneMerchant(ctx, repo.NewReqOneMerchant(account))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", status.Error(codes.NotFound, "商户不存在")
	}
	if err != nil {
		return "", status.Error(codes.Unavailable, "读取商户失败")
	}
	key := []byte(config.Get().EncryptKey)
	ciphertext := merchant.NotifySecret
	if ciphertext == "" {
		secret, err := helpers.GenerateSecret()
		if err != nil {
			return "", status.Error(codes.Internal, "生成回调密钥失败")
		}
		ciphertext, err = helpers.EncryptPEMString(secret, key)
		if err != nil {
			return "", status.Error(codes.Internal, "加密回调密钥失败")
		}
		ciphertext, err = mysql.InitNotifySecret(ctx, repo.NewReqNotifySecret(
			repo.WithReqNotifySecretAccount(account),
			repo.WithReqNotifySecretSecret(ciphertext),
		))
		if err != nil {
			return "", status.Error(codes.Unavailable, "保存回调密钥失败")
		}
	}
	secret, err := helpers.DecryptPEMString(ciphertext, key)
	if err != nil || secret == "" {
		return "", status.Error(codes.Internal, "解密回调密钥失败")
	}
	return secret, nil
}
