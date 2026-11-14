package initials

import (
	"sync"

	"payment-api/internal/config"
	"payment-api/internal/models"
	"payment-api/internal/pkg/gormhelper"
)

var databaseMu sync.Mutex
var databaseConfig config.DataConfig

// InitDatabase 复用 Data 构造入口，连接成功后更新共享客户端。
func InitDatabase(cfg config.DataConfig) error {
	databaseMu.Lock()
	defer databaseMu.Unlock()
	if cfg == databaseConfig {
		return nil
	}
	if cfg == (config.DataConfig{}) {
		gormhelper.ClientInstance.Suspend()
		databaseConfig = cfg
		return nil
	}
	data := models.NewData(&cfg)
	db, err := data.LoadDatabase(models.MainDataBase)
	if err != nil {
		return err
	}
	if db.Error != nil {
		return db.Error
	}
	databaseConfig = cfg
	return nil
}
