package rediscache

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/gomodule/redigo/redis"
)

type RedisConfig struct {
	MaxActive   int
	MaxIdle     int
	Wait        bool
	Host        string
	Port        int
	Password    string
	DbNum       int
	Timeout     int
	RequirePass bool
}

var CacheClientInstance atomic.Pointer[CacheClient]

func NewRedisCacheClient(client CacheClient) {
	CacheClientInstance.Store(&client)
}

func RedisCacheClient(cfg RedisConfig) CacheClient {
	cfg.Timeout = cfg.Timeout * 2
	handler := &redis.Pool{
		MaxActive:   cfg.MaxActive,
		MaxIdle:     cfg.MaxIdle,
		IdleTimeout: time.Duration(cfg.Timeout) * time.Second,
		Wait:        cfg.Wait,
		DialContext: func(ctx context.Context) (redis.Conn, error) {
			var conn redis.Conn
			var err error
			if cfg.RequirePass == true {
				conn, err = redis.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
					redis.DialPassword(cfg.Password),
					redis.DialDatabase(cfg.DbNum),
					redis.DialConnectTimeout(time.Duration(cfg.Timeout)*time.Second),
					redis.DialReadTimeout(time.Duration(cfg.Timeout)*time.Second),
					redis.DialWriteTimeout(time.Duration(cfg.Timeout)*time.Second))
			} else {
				conn, err = redis.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
					redis.DialDatabase(cfg.DbNum),
					redis.DialConnectTimeout(time.Duration(cfg.Timeout)*time.Second),
					redis.DialReadTimeout(time.Duration(cfg.Timeout)*time.Second),
					redis.DialWriteTimeout(time.Duration(cfg.Timeout)*time.Second))
			}
			if err != nil {
				log.Println("Redis 初始化失败", err)
				if conn != nil {
					_ = conn.Close()
				}
				return nil, err
			}
			return conn, nil
		},
	}
	return CacheClient{pool: handler}
}

type CacheClient struct {
	pool *redis.Pool
}

func (c *CacheClient) DoContext(ctx context.Context, command string, args ...interface{}) (interface{}, error) {
	if c == nil || c.pool == nil {
		return nil, errors.New("Redis 未初始化")
	}
	client, err := c.pool.GetContext(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	return redis.DoContext(client, ctx, command, args...)
}

// Do executes a Redis command for typed features that are not covered by the
// cache helpers, such as Streams. Callers must use Redis command arguments.
func (c *CacheClient) Do(command string, args ...interface{}) (interface{}, error) {
	client := c.pool.Get()
	defer client.Close()
	if client.Err() != nil {
		return nil, client.Err()
	}
	return client.Do(command, args...)
}

func (c *CacheClient) Set(cacheName string, value interface{}, expire ...int) error {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return client.Err()
	}
	if len(expire) > 0 {
		_, err = client.Do("SETEX", cacheName, expire[0], value)
	} else {
		_, err = client.Do("SET", cacheName, value)
	}

	return err
}

// SetNx 若不存在设置缓存
func (c *CacheClient) SetNx(cacheName string, value interface{}, expire ...int) error {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return client.Err()
	}
	if len(expire) > 0 {
		_, err = redis.String(client.Do("SET", cacheName, value, "EX", expire[0], "NX"))
	} else {
		_, err = redis.Int(client.Do("SETNX", cacheName, value))
	}
	if err != nil {
		return err
	}
	if err == redis.ErrNil {
		return errors.New("cache Key Existed")
	}
	return nil
}

// Get 获取缓存
func (c *CacheClient) Get(cacheName string) (interface{}, error) {
	client := c.pool.Get()
	defer client.Close()
	if client.Err() != nil {
		return nil, client.Err()
	}
	result, err := client.Do("GET", cacheName)
	return result, err
}

// Incr 设置缓存自增
func (c *CacheClient) Incr(cacheName string, expire ...int) (int64, error) {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return 0, client.Err()
	}
	keyExist, err := client.Do("GET", cacheName)
	if keyExist == nil {
		_, _ = redis.Int64(client.Do("SET", cacheName, 1, "EX", expire[0], "NX"))
		return 1, nil
	}
	result, err := redis.Int64(client.Do("INCR", cacheName))
	return result, err
}

// TTL 获取缓存有效期
func (c *CacheClient) TTL(cacheName string) (int64, error) {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return 0, client.Err()
	}
	result, err := redis.Int64(client.Do("TTL", cacheName))
	return result, err
}

// Expire 设置缓存有效期
func (c *CacheClient) Expire(cacheName string, expire int64) error {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return client.Err()
	}
	_, err = redis.Int64(client.Do("EXPIRE", cacheName, expire))
	return err
}

// LLen 获取列表缓存长度
func (c *CacheClient) LLen(cacheName string) (int64, error) {
	client := c.pool.Get()
	defer client.Close()
	if client.Err() != nil {
		return 0, client.Err()
	}
	listLen, err := client.Do("LLEN", cacheName)
	if err != nil {
		return 0, err
	}
	result, err := redis.Int64(listLen, nil)
	return result, err
}

// LPush 设置列表缓存
func (c *CacheClient) LPush(cacheName string, valuesStr []string, expire ...int) error {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return client.Err()
	}
	if len(valuesStr) > 0 {
		client.Do("MULTI")
		for _, v := range valuesStr {
			client.Do("LPUSH", cacheName, v)
		}
		if len(expire) > 0 {
			client.Do("EXPIRE", cacheName, expire[0])
		}
		_, err = client.Do("EXEC")
	} else {
		err = errors.New("len of values <= 0")
	}
	return err
}

// RPush 设置列表缓存
func (c *CacheClient) RPush(cacheName string, valuesStr []string, expire ...int) error {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return client.Err()
	}
	if len(valuesStr) > 0 {
		client.Do("MULTI")
		for _, v := range valuesStr {
			client.Do("RPUSH", cacheName, v)
		}
		if len(expire) > 0 {
			client.Do("EXPIRE", cacheName, expire[0])
		}
		_, err = client.Do("EXEC")
	} else {
		err = errors.New("len of values <= 0")
	}
	return err
}

// LPop 左边删除列表数据并返回值
func (c *CacheClient) LPop(cacheName string) (interface{}, error) {
	client := c.pool.Get()
	defer client.Close()
	if client.Err() != nil {
		return nil, client.Err()
	}
	result, err := client.Do("LPOP", cacheName)
	return result, err
}

// LRem 从列表中删除指定数量的元素，返回实际删除的数量
func (c *CacheClient) LRem(cacheName string, count int, value string) (int, error) {
	client := c.pool.Get()
	defer client.Close()
	if client.Err() != nil {
		return 0, client.Err()
	}
	res, err := redis.Int(client.Do("LREM", cacheName, count, value))
	return res, err
}

// Rpop 右边删除列表数据并返回值
func (c *CacheClient) Rpop(cacheName string) (interface{}, error) {
	client := c.pool.Get()
	defer client.Close()
	if client.Err() != nil {
		return nil, client.Err()
	}
	result, err := client.Do("RPOP", cacheName)
	return result, err
}

// Lrange 根据范围获取列表数据
func (c *CacheClient) Lrange(cacheName string, startRow, listRows int64) ([]interface{}, error) {
	client := c.pool.Get()
	defer client.Close()
	if client.Err() != nil {
		return nil, client.Err()
	}
	result, err := redis.Values(client.Do("LRANGE", cacheName, startRow, listRows))
	return result, err
}

// HSet 设置哈希表
func (c *CacheClient) HSet(cacheName string, field string, value interface{}, expire ...int) error {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return client.Err()
	}
	client.Do("MULTI")
	client.Do("HSET", cacheName, field, value)
	if len(expire) > 0 {
		client.Do("EXPIRE", cacheName, expire[0])
	}
	_, err = client.Do("EXEC")
	return err
}

// HSetMany 批量设置哈希表
func (c *CacheClient) HSetMany(cacheName string, data map[string]interface{}, expire ...int) error {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return client.Err()
	}
	if len(data) == 0 {
		return errors.New("nil data")
	}
	_, _ = client.Do("MULTI")
	for k, v := range data {
		_, _ = client.Do("HSET", cacheName, k, v)
	}
	if len(expire) > 0 {
		_, _ = client.Do("EXPIRE", cacheName, expire[0])
	}
	_, err = client.Do("EXEC")
	return err
}

// HGet 获取哈希表指定字段值
func (c *CacheClient) HGet(cacheName string, field string) (interface{}, error) {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return nil, client.Err()
	}
	result, err := client.Do("HGET", cacheName, field)
	return result, err
}

func (c *CacheClient) HMGet(cacheName string, fields ...string) (interface{}, error) {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return nil, client.Err()
	}
	result, err := client.Do("HMGET", redis.Args{cacheName}.AddFlat(fields)...)
	return result, err
}

// Exists 判断键值是否存在
func (c *CacheClient) Exists(cacheName string) (int, error) {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return 0, client.Err()
	}
	result, err := redis.Int(client.Do("EXISTS", cacheName))
	return result, err
}

// GetByte 获取byte
func (c *CacheClient) GetByte(cacheName string) ([]byte, error) {
	client := c.pool.Get()
	defer client.Close()
	if client.Err() != nil {
		return nil, client.Err()
	}
	result, err := redis.Bytes(client.Do("GET", cacheName))
	return result, err
}

// HExists 判断哈希表中指定字段是否存在
func (c *CacheClient) HExists(cacheName string, field string) (bool, error) {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return false, client.Err()
	}
	result, err := redis.Bool(client.Do("HEXISTS", cacheName, field))
	return result, err
}

// HDel 删除哈希表中指定字段
func (c *CacheClient) HDel(cacheName string, field string) (bool, error) {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return false, client.Err()
	}
	result, err := redis.Bool(client.Do("HDEL", cacheName, field))
	return result, err
}

// HGetAll 获取哈希表的所有值
func (c *CacheClient) HGetAll(cacheName string) (*map[string]string, error) {
	client := c.pool.Get()
	var err error
	defer client.Close()
	if client.Err() != nil {
		return nil, client.Err()
	}
	result, err := redis.StringMap(client.Do("HGETALL", cacheName))
	return &result, err
}

func (c *CacheClient) HIncrBy(cacheName string, field string, by int) (int, error) {
	client := c.pool.Get()
	defer client.Close()
	var err = client.Err()
	if err != nil {
		return 0, err
	}
	result, err := redis.Int64(client.Do("HINCRBY", cacheName, field, by))
	return int(result), err
}

// Del 删除缓存
func (c *CacheClient) Del(cacheName string) error {
	client := c.pool.Get()
	defer client.Close()
	if client.Err() != nil {
		return nil
	}
	_, err := client.Do("DEL", cacheName)
	return err
}

func (c *CacheClient) DelMul(cacheNames ...string) error {
	client := c.pool.Get()
	defer client.Close()
	if client.Err() != nil {
		return nil
	}
	_, err := client.Do("DEL", redis.Args{}.AddFlat(cacheNames)...)
	return err
}

// IncrBy 缓存
func (c *CacheClient) IncrBy(cacheName string, incr int) (int64, error) {
	client := c.pool.Get()
	var err error
	defer client.Close()
	reply, err := redis.Int64(client.Do("INCRBY", cacheName, incr))
	if err != nil {
		return 0, err
	}
	return reply, err
}

func (c *CacheClient) SAdd(cacheName string, members ...string) error {
	client := c.pool.Get()
	var err error
	defer client.Close()
	_, err = client.Do("SADD", redis.Args{cacheName}.AddFlat(members)...)
	return err
}

func (c *CacheClient) SMembers(cacheName string) ([]string, error) {
	client := c.pool.Get()
	defer client.Close()
	return redis.Strings(client.Do("SMEMBERS", cacheName))
}

func (c *CacheClient) SREM(cacheName string, fields ...string) error {
	client := c.pool.Get()
	defer client.Close()
	_, err := client.Do("SREM", redis.Args{cacheName}.AddFlat(fields)...)
	return err
}

func (c *CacheClient) ZREMRANGEBYSCORE(cache string, min int) error {
	client := c.pool.Get()
	defer client.Close()
	_, err := client.Do("ZREMRANGEBYSCORE", cache, 0, min)
	return err
}

func (c *CacheClient) ZRANGEBYSCORES(cache string) (map[string]int64, error) {
	client := c.pool.Get()
	defer client.Close()
	return redis.Int64Map(client.Do("ZRANGE", cache, 0, -1, "WITHSCORES"))
}

func (c *CacheClient) ZRANGEBYSCORESV0(cache string, min int, max int) (map[string]int64, error) {
	client := c.pool.Get()
	defer client.Close()
	return redis.Int64Map(client.Do("ZRANGEBYSCORE", cache, min, max, "WITHSCORES"))
}

func (c *CacheClient) ZRANGEBYSCORESV1(cache string) (map[string]float64, error) {
	client := c.pool.Get()
	defer client.Close()
	return redis.Float64Map(client.Do("ZRANGE", cache, 0, -1, "WITHSCORES"))
}

func (c *CacheClient) ZScore(cache string, member string) (int64, error) {
	client := c.pool.Get()
	defer client.Close()
	return redis.Int64(client.Do("ZSCORE", cache, member))
}

func (c *CacheClient) ZScoreV1(cache string, member string) (float64, error) {
	client := c.pool.Get()
	defer client.Close()
	return redis.Float64(client.Do("ZSCORE", cache, member))
}

func (c *CacheClient) ZADD(cache string, fields ...interface{}) error {
	client := c.pool.Get()
	if client == nil {
		return errors.New("redis client is nil")
	}
	defer client.Close()
	_, err := client.Do("ZADD", redis.Args{}.Add(cache).AddFlat(fields)...)
	return err
}

func (c *CacheClient) ZREM(cache string, field string) error {
	client := c.pool.Get()
	defer client.Close()
	_, err := client.Do("ZREM", cache, field)
	return err
}

// ZMSCORE Since 6.2.0
func (c *CacheClient) ZMSCORE(cache string, fields ...interface{}) ([]int64, error) {
	client := c.pool.Get()
	defer client.Close()
	return redis.Int64s(client.Do("ZMSCORE", redis.Args{}.Add(cache).AddFlat(fields)...))
}

func (c *CacheClient) ZMSCOREV0(cache string, fields []string) ([]int64, error) {
	client := c.pool.Get()
	defer client.Close()
	var results = make([]int64, len(fields))
	for index, field := range fields {
		r, err := redis.Int64(client.Do("ZSCORE", cache, field))
		if err != nil {
			break
		}
		results[index] = r
	}
	return results, nil
}

func (c *CacheClient) ZCARD(cache string) (int, error) {
	client := c.pool.Get()
	defer client.Close()
	return redis.Int(client.Do("ZCARD", cache))
}

func (c *CacheClient) ZSCAN(cache string, match string, count int, f func(map[string]int64)) error {
	client := c.pool.Get()
	defer client.Close()
	var (
		cursor int64 = 0
	)
	for {
		reply, err := client.Do("ZSCAN", cache, cursor, "MATCH", match, "COUNT", count)
		if err != nil {
			return err
		}
		arr, err := redis.Values(reply, nil)
		if err != nil {
			return err
		}
		cursor, err = redis.Int64(arr[0], nil)
		if err != nil {
			return err
		}
		data, err := redis.Int64Map(arr[1], nil)
		if err != nil {
			return err
		}
		f(data)
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (c *CacheClient) SetBit(cache string, index int) error {
	client := c.pool.Get()
	defer client.Close()
	_, err := client.Do("SETBIT", cache, index, 1)
	return err
}

func (c *CacheClient) GetBit(cache string, index int) (int, error) {
	client := c.pool.Get()
	defer client.Close()
	return redis.Int(client.Do("GETBIT", cache, index))
}
func (c *CacheClient) DelIfMatch(cacheName string, value interface{}) error {
	client := c.pool.Get()
	defer client.Close()
	if client.Err() != nil {
		return client.Err()
	}
	script := "if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end"
	_, err := redis.Int(client.Do("EVAL", script, 1, cacheName, value))
	return err
}
