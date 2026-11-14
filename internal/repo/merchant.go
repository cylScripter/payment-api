package repo

import (
	"payment-api/internal/models"

	"gorm.io/gorm"
)

type MerchantMysql struct {
	db *gorm.DB
}

func NewMerchantMysql(db *gorm.DB) *MerchantMysql {
	return &MerchantMysql{db: db}
}

func (m *MerchantMysql) Merchants() []*models.PayMerchant {
	list := make([]*models.PayMerchant, 0)
	m.db.Model(&models.PayMerchant{}).Where("status = 1").Find(&list)
	return list
}

func (m *MerchantMysql) MerchantChannels() []*models.PayMerchantChannel {
	list := make([]*models.PayMerchantChannel, 0)
	m.db.Model(&models.PayMerchantChannel{}).Where("status = 1").Find(&list)
	return list
}
