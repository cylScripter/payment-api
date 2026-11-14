package repo

import (
	"context"
	"errors"
	"payment-api/internal/models"

	"gorm.io/gorm"
)

type PayMysql struct {
	db *gorm.DB
}

func NewPayMysql(db *gorm.DB) *PayMysql {
	return &PayMysql{db: db}
}

func (m *PayMysql) OrderExists(payNo string) (*models.PayOrder, error) {
	var doc models.PayOrder
	err := m.db.Where("pay_no = ?", payNo).First(&doc).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &doc, nil
}

func (m *PayMysql) ToOrder(ctx context.Context, payOrder *models.PayOrder) error {
	return m.db.WithContext(ctx).Create(payOrder).Error
}

func (m *PayMysql) UpdateOrder(payNo string, updateMap map[string]interface{}) error {
	return m.db.Model(&models.PayOrder{}).Where("pay_no = ?", payNo).Updates(updateMap).Error
}
