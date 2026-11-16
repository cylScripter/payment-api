package helpers

import "time"

// ParseTime 解析 RFC3339 时间字符串，保留字符串中的时区偏移。
func ParseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, value)
}
