package repo

import (
	"payment-api/internal/models"

	"gorm.io/gorm"
)

type MysqlFLow struct {
	db *gorm.DB
}

func NewMysqlFlow(db *gorm.DB) *MysqlFLow {
	return &MysqlFLow{db: db}
}

type ReqOneFlow struct {
	tradeNo string
}

func NewReqOneFlow(tradeNo string) *ReqOneFlow {
	return &ReqOneFlow{tradeNo: tradeNo}
}

func (m *MysqlFLow) OneFlow(req *ReqOneFlow) models.PayFlow {
	var doc models.PayFlow
	db := m.db.Model(&models.PayFlow{})
	if req.tradeNo != "" {
		db = db.Where("trade_no = ?", req.tradeNo)
	}
	db.First(&doc)
	return doc
}

func (m *MysqlFLow) ToFlow(flow *models.PayFlow) error {
	return m.db.Create(flow).Error
}
