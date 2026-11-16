package repo

import (
	"context"
	"errors"
	"payment-api/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"
)

type PayMysql struct {
	db *gorm.DB
}

func NewPayMysql(db *gorm.DB) *PayMysql {
	return &PayMysql{db: db}
}

func (m *PayMysql) UpdateNotifyStatus(ctx context.Context, payNo, notifyStatus string) error {
	return m.db.WithContext(ctx).Model(&models.PayOrder{}).
		Where("pay_no = ?", payNo).
		Update("business_notify_status", notifyStatus).Error
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
	query := m.db.Model(&models.PayOrder{}).Where("pay_no = ?", payNo)
	if updateMap["status"] == models.PayStatusFailed {
		query = query.Where("status IS NULL OR status <> ?", models.PayStatusSuccess)
	}
	return query.Updates(updateMap).Error
}

// ConfirmOrder 在同一事务中确认支付成功并增加商户钱包余额。
func (m *PayMysql) ConfirmOrder(ctx context.Context, payment *models.PayOrder) error {
	return m.db.WithContext(ctx).Clauses(dbresolver.Write).Transaction(func(tx *gorm.DB) error {
		var order models.PayOrder
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("pay_no = ? AND merchant_account = ?", payment.PayNo, payment.MerchantAccount).
			First(&order).Error
		if err != nil {
			return err
		}
		if order.Status == models.PayStatusSuccess {
			return nil
		}
		// 锁定商户，串行处理该商户的钱包创建和入账。
		var merchant models.PayMerchant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("account = ?", order.MerchantAccount).First(&merchant).Error; err != nil {
			return err
		}
		var wallet models.MerchantWallet
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("account = ?", order.MerchantAccount).First(&wallet).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			wallet = models.MerchantWallet{
				Account:    order.MerchantAccount,
				MerchantID: merchant.ID,
				Balance:    payment.TotalAmount,
				Version:    1,
			}
			if err := tx.Create(&wallet).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if err := tx.Model(&wallet).Updates(map[string]any{
			"balance": gorm.Expr("balance + ?", payment.TotalAmount),
			"version": gorm.Expr("version + 1"),
		}).Error; err != nil {
			return err
		}
		return tx.Model(&order).Updates(map[string]any{
			"status":     models.PayStatusSuccess,
			"trade_no":   payment.TradeNo,
			"pay_amount": payment.PayAmount,
			"pay_time":   payment.PayTime,
			"open_id":    payment.OpenID,
			"version":    gorm.Expr("COALESCE(version, 0) + 1"),
		}).Error
	})
}
