package models

import (
	"payment-api/internal/pkg/gormhelper"
	"payment-api/internal/pkg/rediscache"

	"gorm.io/gorm"
)

var data = &Data{database: gormhelper.ClientInstance}

func GetData() *Data { return data }

func PayDb() *gorm.DB {
	return data.MainDB()
}

func RedisClient() *rediscache.CacheClient {
	return rediscache.CacheClientInstance.Load()
}
