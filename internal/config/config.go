package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

type DataConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	ReplicaHost     string `yaml:"replica_host"`
	ReplicaPort     int32  `yaml:"replica_port"`
	ReplicaUser     string `yaml:"replica_user"`
	ReplicaPassword string `yaml:"replica_password"`
	MaxIdle         int32  `yaml:"max_idle"`
	MaxConn         int32  `yaml:"max_conn"`
}

type RPC struct {
	Addr string `yaml:"addr"`
}

type Wechat struct {
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
}

type Config struct {
	EncryptKey string     `yaml:"encrypt_key"` // 数据加解密口令，加密时通过PBKDF2派生AES密钥。
	RPC        RPC        `yaml:"rpc"`
	Wechat     Wechat     `yaml:"wechat"`
	Database   DataConfig `yaml:"database"`
}

var (
	instance atomic.Pointer[Config]
	mu       sync.Mutex
	source   Source
	content  []byte
	onReload func(Config) error
	watching bool
)

// Init 默认读取 CONFIG_PATH 指定的 YAML，可传入其他配置来源。
func Init(sources ...Source) error {
	if len(sources) > 1 {
		return errors.New("只能指定一个配置来源")
	}
	if len(sources) == 1 {
		return SetSource(sources[0])
	}
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "configs/config.yaml"
	}
	return SetSource(FileSource{Path: path})
}

// SetSource 成功加载并应用配置后切换来源，后续热更新继续使用该来源。
func SetSource(next Source) error {
	if next == nil {
		return errors.New("配置来源不能为空")
	}
	mu.Lock()
	defer mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return reload(ctx, next)
}

func Get() Config {
	if cfg := instance.Load(); cfg != nil {
		return *cfg
	}
	return Config{}
}

// Watch 每秒检查配置。apply 成功后发布新快照，返回函数用于停止并等待退出。
func Watch(apply func(Config) error) (func(), error) {
	mu.Lock()
	if source == nil || watching {
		mu.Unlock()
		return nil, errors.New("配置尚未初始化或已启动监听")
	}
	onReload = apply
	watching = true
	mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		lastError := ""
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mu.Lock()
				loadCtx, stop := context.WithTimeout(ctx, 5*time.Second)
				err := reload(loadCtx, source)
				stop()
				mu.Unlock()
				if ctx.Err() != nil {
					return
				}
				if err != nil {
					if err.Error() != lastError {
						slog.Error("配置更新失败，继续使用当前配置", "error", err)
					}
					lastError = err.Error()
					continue
				}
				lastError = ""
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-done
			mu.Lock()
			onReload = nil
			watching = false
			mu.Unlock()
		})
	}, nil
}

func reload(ctx context.Context, next Source) error {
	data, err := next.Load(ctx)
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}
	if instance.Load() != nil && bytes.Equal(data, content) {
		source = next
		return nil
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return errors.New("配置内容不能为空")
	}
	var cfg Config
	loader := viper.New()
	loader.SetConfigType("yaml")
	if err := loader.ReadConfig(bytes.NewReader(data)); err != nil {
		return errors.New("YAML 配置格式错误")
	}
	if err := loader.UnmarshalExact(&cfg, func(options *mapstructure.DecoderConfig) {
		options.TagName = "yaml"
		options.WeaklyTypedInput = false
	}); err != nil {
		return errors.New("配置字段名称或类型错误")
	}
	if cfg.RPC.Addr == "" {
		cfg.RPC.Addr = ":9090"
	}
	db := &cfg.Database
	if *db != (DataConfig{}) {
		if db.Port == 0 {
			db.Port = 3306
		}
		if db.ReplicaHost != "" {
			if db.ReplicaPort == 0 {
				db.ReplicaPort = 3306
			}
			if db.ReplicaUser == "" {
				db.ReplicaUser = db.User
				db.ReplicaPassword = db.Password
			}
		}
	}
	if onReload != nil {
		if err := onReload(cfg); err != nil {
			return err
		}
	}
	instance.Store(&cfg)
	source = next
	content = bytes.Clone(data)
	return nil
}
