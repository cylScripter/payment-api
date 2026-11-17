package initials

import (
	"context"
	"encoding/json"
	"payment-api/internal/models"
	"payment-api/internal/repo"
	"time"

	log "github.com/sirupsen/logrus"
)

func startAPIKeySync() (func(), error) {
	ctx, cancel := context.WithCancel(context.Background())
	if err := syncAPIKeys(ctx); err != nil {
		cancel()
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		// ponytail: 商户状态每 30 秒同步，需即时生效时接入商户变更后的主动更新。
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := syncAPIKeys(ctx); err != nil && ctx.Err() == nil {
					log.WithError(err).Error("同步 API_KEY 鉴权缓存失败")
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}, nil
}

func syncAPIKeys(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	keys, err := repo.NewMySQLAPIKey(models.PayDb()).List(ctx)
	if err != nil {
		return err
	}
	client := models.RedisClient()
	for _, key := range keys {
		data, err := json.Marshal(key.PayMerchant)
		if err != nil {
			return err
		}
		if _, err := client.DoContext(ctx, "SET", models.APIKeyCachePrefix+key.KeyHash, data, "EX", models.APIKeyCacheTTL); err != nil {
			return err
		}
	}
	// 预留单次同步的 30 秒超时，确保就绪标记先于本批密钥失效。
	_, err = client.DoContext(ctx, "SET", models.APIKeyCachePrefix+"ready", "1", "EX", models.APIKeyCacheTTL-30)
	return err
}
