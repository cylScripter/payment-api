package gormhelper

import (
	"errors"
	"fmt"
	"net/url"
	"sync"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type MysqlConfig struct {
	MasterSource  string
	ReplicaSource string
	MaxIdle       int
	MaxConn       int
}

// Client 保留按名称管理数据库的入口。
type Client struct{ C sync.Map }

// ClientInstance 地址固定，Data 实例通过此入口获取当前数据库。
var ClientInstance = &Client{}
var clientMu sync.Mutex

func NewClient(client *Client) {
	clientMu.Lock()
	defer clientMu.Unlock()
	if client.Err() != nil {
		_ = client.closePools()
		return
	}
	client.C.Range(func(key, value interface{}) bool {
		next := value.(*gorm.DB)
		if previous, ok := ClientInstance.C.Load(key); ok {
			oldDB := previous.(*gorm.DB)
			oldPool, newPool := connectionFor(oldDB), connectionFor(next)
			if oldPool != newPool {
				oldPool.adopt(newPool)
				connections.Delete(next)
			}
			next = oldDB
			client.C.Store(key, next)
		}
		ClientInstance.C.Store(key, next)
		return true
	})
}

// MysqlClient 使用 database 指定实际连接库名和连接池名称。
func MysqlClient(database string, cfg MysqlConfig, log logger.Interface) *gorm.DB {
	master, err := mysqlDriver.ParseDSN(fmt.Sprintf("%s/%s?charset=utf8mb4&parseTime=true&loc=Asia%%2FShanghai&timeout=5s&readTimeout=10s&writeTimeout=10s", cfg.MasterSource, url.PathEscape(database)))
	if err != nil {
		return &gorm.DB{Error: errors.New("MySQL 主库配置格式错误")}
	}
	connector, err := mysqlDriver.NewConnector(master)
	if err != nil {
		return &gorm.DB{Error: fmt.Errorf("创建 MySQL 主库配置失败: %w", err)}
	}
	options := poolConfig{Master: connector, MaxIdle: cfg.MaxIdle, MaxConn: cfg.MaxConn}
	if cfg.ReplicaSource != "" {
		replica, parseErr := mysqlDriver.ParseDSN(fmt.Sprintf("%s/%s?charset=utf8mb4&parseTime=true&loc=Asia%%2FShanghai&timeout=5s&readTimeout=10s&writeTimeout=10s", cfg.ReplicaSource, url.PathEscape(database)))
		if parseErr != nil {
			return &gorm.DB{Error: errors.New("MySQL 从库配置格式错误")}
		}
		options.Replica, err = mysqlDriver.NewConnector(replica)
		if err != nil {
			return &gorm.DB{Error: fmt.Errorf("创建 MySQL 从库配置失败: %w", err)}
		}
	}
	clientMu.Lock()
	defer clientMu.Unlock()
	if previous, ok := ClientInstance.C.Load(database); ok {
		next, err := openConnections(options)
		if err != nil {
			return &gorm.DB{Error: err}
		}
		db := previous.(*gorm.DB)
		connectionFor(db).adopt(next)
		return db
	}
	db, err := openMySQL(options, log)
	if err != nil {
		return &gorm.DB{Error: err}
	}
	return db
}

func (c *Client) Err() error {
	var result error
	c.C.Range(func(_, value interface{}) bool { result = errors.Join(result, value.(*gorm.DB).Error); return true })
	return result
}

func (c *Client) Close() error {
	clientMu.Lock()
	defer clientMu.Unlock()
	err := c.closePools()
	c.C.Range(func(key, _ interface{}) bool { c.C.Delete(key); return true })
	return err
}

func (c *Client) Suspend() {
	clientMu.Lock()
	defer clientMu.Unlock()
	c.C.Range(func(_, value interface{}) bool { connectionFor(value.(*gorm.DB)).configure(&poolConfig{}); return true })
}

func (c *Client) closePools() error {
	var result error
	c.C.Range(func(_, value interface{}) bool {
		db := value.(*gorm.DB)
		if owner, ok := connections.LoadAndDelete(db); ok {
			result = errors.Join(result, owner.(*connection).Close())
		}
		return true
	})
	return result
}

func connectionFor(db *gorm.DB) *connection {
	owner, _ := connections.Load(db)
	return owner.(*connection)
}
