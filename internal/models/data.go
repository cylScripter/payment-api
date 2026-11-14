package models

import (
	"payment-api/internal/pkg/gormhelper"

	"gorm.io/gorm"
)

var data = &Data{database: gormhelper.ClientInstance}

func GetData() *Data { return data }

func PayDb() *gorm.DB {
	return data.MainDB()
}
