package repo

import (
	"context"
	"payment-api/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"
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

func (m *MerchantMysql) OneMerchant(ctx context.Context, req *ReqOneMerchant) models.PayMerchant {
	var doc models.PayMerchant
	db := m.db.WithContext(ctx).Clauses(dbresolver.Write).Model(&models.PayMerchant{})
	if req.account != "" {
		db = db.Where("account = ?", req.account)
	}
	db.First(&doc)
	return doc
}

type ReqNotifySecret struct {
	account string
	secret  string
}

func NewReqNotifySecret(opts ...func(*ReqNotifySecret)) ReqNotifySecret {
	var req ReqNotifySecret
	for _, opt := range opts {
		opt(&req)
	}
	return req
}

func WithReqNotifySecretAccount(account string) func(*ReqNotifySecret) {
	return func(req *ReqNotifySecret) {
		req.account = account
	}
}

func WithReqNotifySecretSecret(secret string) func(*ReqNotifySecret) {
	return func(req *ReqNotifySecret) {
		req.secret = secret
	}
}

func (m *MerchantMysql) NotifyMerchant(ctx context.Context, req ReqNotifySecret) (models.PayMerchant, error) {
	var merchant models.PayMerchant
	err := m.db.WithContext(ctx).Clauses(dbresolver.Write).
		Where("account = ?", req.account).First(&merchant).Error
	return merchant, err
}

func (m *MerchantMysql) InitNotifySecret(ctx context.Context, req ReqNotifySecret) (string, error) {
	merchant, err := m.NotifyMerchant(ctx, req)
	if err != nil {
		return "", err
	}
	// 密文写入时关闭 SQL 输出，避免密钥材料进入日志。
	err = m.db.WithContext(ctx).Session(&gorm.Session{Logger: m.db.Logger.LogMode(logger.Silent)}).
		Model(&models.PayMerchant{}).
		Where("id = ? AND notify_secret = ''", merchant.ID).
		Update("notify_secret", req.secret).Error
	if err != nil {
		return "", err
	}
	err = m.db.WithContext(ctx).Clauses(dbresolver.Write).First(&merchant, merchant.ID).Error
	return merchant.NotifySecret, err
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
