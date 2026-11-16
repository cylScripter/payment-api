package models

import "time"

type MerchantWallet struct {
	ID         uint64    `gorm:"primaryKey"`
	CreateTime time.Time `json:"create_time" gorm:"column:create_time;not null;autoCreateTime"` // 创建时间                                            // 本地入库时间
	UpdateTime time.Time `json:"update_time" gorm:"column:update_time;not null;autoUpdateTime"` // 更新时间
	Account    string    `json:"account" gorm:"column:account;not null"`                        // 商户账号
	MerchantID int64     `json:"merchant_id" gorm:"column:merchant_id;not null"`                // 商户id
	Balance    int64     `json:"balance" gorm:"column:balance;not null;default:0"`
	Version    int64     `json:"version" gorm:"column:version;not null;default:0"`
}

func (MerchantWallet) TableName() string {
	return "merchant_wallet"
}
