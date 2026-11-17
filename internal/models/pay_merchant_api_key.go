package models

import "time"

const APIKeyCachePrefix = "payment-api:auth:api-key:"
const APIKeyCacheTTL = 120

type PayMerchantAPIKey struct {
	ID         int64     `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	MerchantID int64     `json:"merchant_id" gorm:"column:merchant_id;not null;index:idx_merchant_id"`
	KeyHash    string    `json:"-" gorm:"column:key_hash;type:char(64);not null;uniqueIndex:uk_key_hash"`
	CreateTime time.Time `json:"create_time" gorm:"column:create_time;type:datetime(3);not null;autoCreateTime"`
}

func (PayMerchantAPIKey) TableName() string { return "pay_merchant_api_key" }
