package models

import (
	"errors"
	"fmt"
	"payment-api/internal/config"
	"payment-api/internal/pkg/gormhelper"
	"payment-api/internal/pkg/gormlogger"
	"payment-api/internal/pkg/rediscache"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	MainDataBase = "jia_ye_pay"
)

type Data struct {
	database *gormhelper.Client
	redis    rediscache.CacheClient
}

func (d *Data) LoadDatabase(database string) (*gorm.DB, error) {
	v, ok := d.database.C.Load(database)
	if ok {
		return v.(*gorm.DB), nil
	}
	return nil, errors.New("invalid Gorm db")
}

func (d *Data) MainDB() *gorm.DB {
	db, _ := d.LoadDatabase(MainDataBase)
	return db
}

func NewDataBaseClient(c *config.DataConfig, log logger.Interface) *gormhelper.Client {
	cfg := gormhelper.MysqlConfig{
		MaxConn:       int(c.MaxConn),
		MaxIdle:       int(c.MaxIdle),
		MasterSource:  fmt.Sprintf("%s:%s@tcp(%s:%d)", c.User, c.Password, c.Host, c.Port),
		ReplicaSource: fmt.Sprintf("%s:%s@tcp(%s:%d)", c.ReplicaUser, c.ReplicaPassword, c.ReplicaHost, c.ReplicaPort),
	}
	if c.ReplicaHost == "" {
		cfg.ReplicaSource = ""
	}
	dbs := []string{MainDataBase}
	var client = new(gormhelper.Client)
	for index := range dbs {
		db := dbs[index]
		cc := gormhelper.MysqlClient(db, cfg, log)
		client.C.Store(db, cc)
	}
	gormhelper.NewClient(client)
	return client
}

func NewData(c *config.Config) *Data {
	log := gormlogger.New()
	return &Data{
		database: NewDataBaseClient(&c.Database, log),
		redis:    NewRedisClient(&c.Redis),
	}
}
func (d *Data) Redis() *rediscache.CacheClient {
	return &d.redis
}

func NewRedisClient(c *config.RedisConfig) rediscache.CacheClient {
	cfg := rediscache.RedisConfig{
		MaxActive:   int(c.MaxActive),
		MaxIdle:     int(c.MaxIdle),
		Wait:        c.Wait,
		Host:        c.Host,
		Port:        int(c.Port),
		Password:    c.Password,
		DbNum:       int(c.DbNum),
		Timeout:     int(c.Timeout),
		RequirePass: c.Password != "",
	}
	client := rediscache.RedisCacheClient(cfg)
	rediscache.NewRedisCacheClient(client)
	return client
}
