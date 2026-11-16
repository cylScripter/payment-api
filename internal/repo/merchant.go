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

type ReqOneMerchant struct {
	account string
}

func NewReqOneMerchant(account string) *ReqOneMerchant {
	return &ReqOneMerchant{account: account}
}

func (m *MerchantMysql) OneMerchant(req *ReqOneMerchant) models.PayMerchant {
	var doc models.PayMerchant
	db := m.db.Model(&models.PayMerchant{})
	if req.account != "" {
		db = db.Where("account = ?", req.account)
	}
	db.First(&doc)
	return doc
}

type ReqOneMerchantChannel struct {
	merchantId  int64
	channelCode string
}

func NewReqOneMerchantChannel(merchantId int64, channelCode string) *ReqOneMerchantChannel {
	return &ReqOneMerchantChannel{
		merchantId:  merchantId,
		channelCode: channelCode,
	}
}

func (m *MerchantMysql) OneMerchantChannel(req *ReqOneMerchantChannel) models.PayMerchantChannel {
	var doc models.PayMerchantChannel
	db := m.db.Model(&models.PayMerchantChannel{})
	if req.merchantId != 0 {
		db = db.Where("merchant_id = ?", req.merchantId)
	}
	if req.channelCode != "" {
		db = db.Where("channel_code = ?", req.channelCode)
	}
	db.First(&doc)
	return doc
}
