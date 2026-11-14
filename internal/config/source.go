package config

import (
	"context"
	"os"
)

// Source 提供 YAML 内容，可实现为文件、配置中心或其他存储。
type Source interface {
	Load(context.Context) ([]byte, error)
}

type FileSource struct{ Path string }

func (s FileSource) Load(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.ReadFile(s.Path)
}
