package helpers

import "github.com/shopspring/decimal"

// YuanToFen 将 float32 元金额转换为 int64 分，超过两位小数时四舍五入。
// amount 须为有限数值，换算结果须在 int64 范围内。
func YuanToFen(amount float32) int64 {
	return decimal.NewFromFloat32(amount).Shift(2).Round(0).IntPart()
}
