package repo

import (
	"context"
	"errors"
	"payment-api/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"
)

type MySQLAPIKey struct {
	db *gorm.DB
}

func NewMySQLAPIKey(db *gorm.DB) *MySQLAPIKey {
	return &MySQLAPIKey{db: db}
}

type ReqAPIKey struct {
	keyHash string
}

func NewReqAPIKey(opts ...func(*ReqAPIKey)) ReqAPIKey {
	var req ReqAPIKey
	for _, opt := range opts {
		opt(&req)
	}
	return req
}

func WithReqAPIKeyHash(keyHash string) func(*ReqAPIKey) {
	return func(req *ReqAPIKey) {
		req.keyHash = keyHash
	}
}

func (m *MySQLAPIKey) One(ctx context.Context, req ReqAPIKey) (models.PayMerchantAPIKey, error) {
	var key models.PayMerchantAPIKey
	err := m.db.WithContext(ctx).Clauses(dbresolver.Write).
		Where("key_hash = ?", req.keyHash).First(&key).Error
	return key, err
}

func (m *MySQLAPIKey) Merchant(ctx context.Context, req ReqAPIKey) (models.PayMerchant, error) {
	var merchant models.PayMerchant
	err := m.db.WithContext(ctx).Clauses(dbresolver.Write).
		Model(&models.PayMerchant{}).
		Select("pay_merchant.*").
		Joins("JOIN pay_merchant_api_key ON pay_merchant_api_key.merchant_id = pay_merchant.id").
		Where("pay_merchant_api_key.key_hash = ?", req.keyHash).
		First(&merchant).Error
	return merchant, err
}

type APIKeyMerchant struct {
	KeyHash string `gorm:"column:key_hash"`
	models.PayMerchant
}

func (m *MySQLAPIKey) List(ctx context.Context) ([]APIKeyMerchant, error) {
	var keys []APIKeyMerchant
	err := m.db.WithContext(ctx).Clauses(dbresolver.Write).
		Model(&models.PayMerchantAPIKey{}).
		Select("pay_merchant_api_key.key_hash, pay_merchant.*").
		Joins("LEFT JOIN pay_merchant ON pay_merchant.id = pay_merchant_api_key.merchant_id").
		Scan(&keys).Error
	return keys, err
}

type ReqCreateAPIKey struct {
	account string
	keyHash string
}

func NewReqCreateAPIKey(opts ...func(*ReqCreateAPIKey)) ReqCreateAPIKey {
	var req ReqCreateAPIKey
	for _, opt := range opts {
		opt(&req)
	}
	return req
}

func WithReqCreateAPIKeyAccount(account string) func(*ReqCreateAPIKey) {
	return func(req *ReqCreateAPIKey) {
		req.account = account
	}
}

func WithReqCreateAPIKeyHash(keyHash string) func(*ReqCreateAPIKey) {
	return func(req *ReqCreateAPIKey) {
		req.keyHash = keyHash
	}
}

func (m *MySQLAPIKey) Create(ctx context.Context, req ReqCreateAPIKey) (models.PayMerchant, error) {
	var merchant models.PayMerchant
	err := m.db.WithContext(ctx).Clauses(dbresolver.Write).Transaction(func(tx *gorm.DB) error {
		var merchants []models.PayMerchant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("BINARY account = ?", req.account).Limit(2).Find(&merchants).Error; err != nil {
			return err
		}
		if len(merchants) != 1 || merchants[0].Status != 1 {
			return errors.New("商户不可用")
		}
		merchant = merchants[0]
		key := models.PayMerchantAPIKey{
			MerchantID: merchant.ID,
			KeyHash:    req.keyHash,
		}
		return tx.Create(&key).Error
	})
	return merchant, err
}
