package rediscache

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
)

var cfg = RedisConfig{
	MaxActive:   10,
	MaxIdle:     2,
	Wait:        false,
	Host:        "192.168.102.30",
	Port:        6379,
	Password:    "LeBiedisN321^",
	DbNum:       0,
	Timeout:     10,
	RequirePass: true,
}

func TestRedisAllCommand(t *testing.T) {

	client := RedisCacheClient(cfg)
	err := client.SAdd("hello", "1", "2", "3")
	logrus.Println(err)
	values, _ := client.SMembers("hello")
	logrus.Println(values)
	client.SREM("hello", "1", "2")
	values, _ = client.SMembers("hello")
	logrus.Println(values)

}

func TestRedisHMGet(t *testing.T) {
	client := RedisCacheClient(cfg)
	res, _ := client.HMGet("bike_device_864946068457281", "device_info", "gps_data")
	result := res.([]interface{})
	fmt.Println(result)

	res, _ = client.HMGet("sDueCount:20230724", "sDueAlready:slebei", "sDuesDueInFuture:slebei")
	var d []byte
	var g []byte
	rev := res.([]interface{})
	if len(rev) == 2 {
		if rev[0] != nil {
			d = rev[0].([]byte)
		}
		if rev[1] != nil {
			g = rev[1].([]byte)
		}
	}
	var (
		dueCount    int
		dueInFuture int
	)
	_ = json.Unmarshal(d, &dueCount)
	_ = json.Unmarshal(g, &dueInFuture)
	fmt.Println(dueCount, dueInFuture)
}

func TestRedisZSet(t *testing.T) {
	client := RedisCacheClient(cfg)
	m, err := client.ZRANGEBYSCORES("myset")
	fmt.Println(err)
	for k, v := range m {
		fmt.Println(k, v)
	}
	client.ZREMRANGEBYSCORE("myset", 1)
	elements := []interface{}{
		1000, "member10",
		1000, "member20",
	}
	client.ZADD("myset", elements...)
	client.ZREM("myset", "member20")
}
func TestRedisDelMul(t *testing.T) {
	client := RedisCacheClient(cfg)
	fmt.Println(client.DelMul("a", "b", "c"))
}

func TestRedisZMSCORE(t *testing.T) {
	client := RedisCacheClient(cfg)
	rs, err := client.ZMSCORE("device:online:v2:slebei", "864946068456424", "3456789009876", "123")
	fmt.Println(rs, err)
}

func TestRedisZMSCOREV0(t *testing.T) {
	client := RedisCacheClient(cfg)
	rs, err := client.ZMSCOREV0("device:online:v2:slebei", []string{"864946068456424", "3456789009876", "123"})
	fmt.Println(rs, err)
}

func TestRedisZSCAN(t *testing.T) {
	client := RedisCacheClient(cfg)
	f := func(results map[string]int64) {
		for key, value := range results {
			fmt.Println(key, value)
		}
	}
	err := client.ZSCAN("device:online:v2:slebei", "*", 1, f)
	fmt.Println(err)
}

func TestRedisSetBit(t *testing.T) {
	client := RedisCacheClient(cfg)
	client.SetBit("mybit", 1)
	client.Expire("mybit", 60)
}

func TestRedisZSCORE(t *testing.T) {
	client := RedisCacheClient(cfg)
	fmt.Println(client.ZScore("device:online:v2:slebei", "862710055606332"))
}
