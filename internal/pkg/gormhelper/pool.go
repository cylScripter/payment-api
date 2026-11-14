package gormhelper

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/dbresolver"
)

type poolConfig struct {
	Master  driver.Connector
	Replica driver.Connector
	MaxIdle int
	MaxConn int
}

// connection 的 GORM 和 sql.DB 地址固定，物理连接按配置版本更新。
type connection struct {
	config  atomic.Pointer[poolConfig]
	master  *sql.DB
	replica *sql.DB
}

func openConnections(cfg poolConfig) (*connection, error) {
	client := &connection{}
	client.config.Store(&cfg)
	client.master = sql.OpenDB(&dialConnector{client: client, driver: cfg.Master.Driver()})
	client.replica = sql.OpenDB(&dialConnector{client: client, read: true, driver: cfg.Master.Driver()})
	for _, db := range []*sql.DB{client.master, client.replica} {
		db.SetMaxIdleConns(cfg.MaxIdle)
		db.SetMaxOpenConns(cfg.MaxConn)
		db.SetConnMaxIdleTime(time.Hour)
		db.SetConnMaxLifetime(2 * time.Hour)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := db.PingContext(ctx)
		cancel()
		if err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("检查 MySQL 连接失败: %w", err)
		}
	}
	return client, nil
}

func openMySQL(cfg poolConfig, log logger.Interface) (*gorm.DB, error) {
	client, err := openConnections(cfg)
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: client.master}), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true}, Logger: log,
	})
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("初始化 MySQL ORM 失败: %w", err)
	}
	err = db.Use(dbresolver.Register(dbresolver.Config{
		Replicas: []gorm.Dialector{mysql.New(mysql.Config{Conn: client.replica, SkipInitializeWithVersion: true})},
	}))
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("初始化 MySQL 读写分离失败: %w", err)
	}
	connections.Store(db, client)
	return db, nil
}

var connections sync.Map

// adopt 发布新连接配置并清空旧空闲连接，活跃事务在归还连接时自然退出旧版本。
func (c *connection) adopt(next *connection) {
	c.configure(next.config.Load())
	_ = next.Close()
}

func (c *connection) configure(cfg *poolConfig) {
	c.config.Store(cfg)
	for _, db := range []*sql.DB{c.master, c.replica} {
		db.SetMaxIdleConns(0)
		db.SetMaxOpenConns(cfg.MaxConn)
		db.SetMaxIdleConns(cfg.MaxIdle)
	}
}

func (c *connection) Close() error { return errors.Join(c.master.Close(), c.replica.Close()) }

type dialConnector struct {
	client *connection
	read   bool
	driver driver.Driver
}

func (d *dialConnector) Connect(ctx context.Context) (driver.Conn, error) {
	cfg := d.client.config.Load()
	connector := cfg.Master
	if d.read && cfg.Replica != nil {
		connector = cfg.Replica
	}
	if connector == nil {
		return nil, errors.New("MySQL 尚未配置")
	}
	conn, err := connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	if d.client.config.Load() != cfg {
		_ = conn.Close()
		return nil, driver.ErrBadConn
	}
	return &versionedConn{Conn: conn, client: d.client, version: cfg}, nil
}

func (d *dialConnector) Driver() driver.Driver { return d.driver }

// versionedConn 让 database/sql 在复用或归还旧版本连接时重新建立物理连接。
type versionedConn struct {
	driver.Conn
	client  *connection
	version *poolConfig
}

func (c *versionedConn) IsValid() bool {
	if c.client.config.Load() != c.version {
		return false
	}
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *versionedConn) ResetSession(ctx context.Context) error {
	if c.client.config.Load() != c.version {
		return driver.ErrBadConn
	}
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *versionedConn) Ping(ctx context.Context) error {
	if pinger, ok := c.Conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c *versionedConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if conn, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return conn.PrepareContext(ctx, query)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.Conn.Prepare(query)
}

func (c *versionedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if conn, ok := c.Conn.(driver.ConnBeginTx); ok {
		return conn.BeginTx(ctx, opts)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.Conn.Begin()
}

func (c *versionedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if conn, ok := c.Conn.(driver.ExecerContext); ok {
		return conn.ExecContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *versionedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if conn, ok := c.Conn.(driver.QueryerContext); ok {
		return conn.QueryContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *versionedConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}
