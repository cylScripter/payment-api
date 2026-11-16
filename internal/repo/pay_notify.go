package repo

import (
	"context"
	"payment-api/internal/models"

	"gorm.io/gorm"
)

type MySQLNotifyLog struct {
	db *gorm.DB
}

func NewMySQLNotifyLog(db *gorm.DB) *MySQLNotifyLog {
	return &MySQLNotifyLog{db: db}
}

func (m *MySQLNotifyLog) Create(ctx context.Context, log *models.PayNotifyLog) error {
	return m.db.WithContext(ctx).Create(log).Error
}
