// Package idgen 使用时间、节点号和序列号生成订单号。
// 单节点严格递增，多节点按毫秒时间趋势有序。
package idgen

import (
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"sync"
	"time"
)

const (
	epoch        int64 = 1735689600000 // 2025-01-01 00:00:00 UTC，发布后保持固定。
	nodeBits           = 10
	sequenceBits       = 12
	maxNode      int64 = 1<<nodeBits - 1
	maxSequence  int64 = 1<<sequenceBits - 1
	maxTimestamp int64 = 1<<41 - 1
)

var orderLocation = time.FixedZone("CST", 8*60*60)

// NodeID 根据主机名和进程号计算 0～1023 的节点号，哈希结果存在碰撞概率。
func NodeID() int64 {
	hostname, _ := os.Hostname()
	hash := fnv.New64a()
	_, _ = fmt.Fprintf(hash, "%s:%d", hostname, os.Getpid())
	return int64(hash.Sum64() & uint64(maxNode))
}

// Generator 在一个节点内长期复用，支持并发调用。
// 每个运行实例须使用独立节点号，范围为 0～1023。
// 节点重启复用节点号前，须确保系统时间已超过该节点最后发号的时间。
type Generator struct {
	mu          sync.Mutex
	nodeID      int64
	startTime   time.Time
	startMillis int64
	lastTime    int64
	sequence    int64
}

func New(nodeID int64) (*Generator, error) {
	if nodeID < 0 || nodeID > maxNode {
		return nil, errors.New("节点号须在 0～1023 范围内")
	}
	startTime := time.Now()
	startMillis := startTime.UnixMilli() - epoch
	if startMillis < 0 || startMillis > maxTimestamp {
		return nil, errors.New("当前时间超出 ID 生成范围")
	}
	return &Generator{
		nodeID:      nodeID,
		startTime:   startTime,
		startMillis: startMillis,
		lastTime:    -1,
	}, nil
}

// Next 返回固定 25 位的纯数字订单号，可按字符串排序。
// 格式为北京时间 yyyyMMddHHmmss + 3 位毫秒 + 4 位节点号 + 4 位序列号。
// 序列号从 0001 开始，同一节点每毫秒最多生成 4095 个 ID，序列用尽后等待下一毫秒。
// 使用单调时钟计算运行时长，系统时间回拨不影响进程内递增。
// 41 位时间耗尽时 panic，避免生成重复 ID。
func (g *Generator) Next() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	for {
		timestamp := g.startMillis + time.Since(g.startTime).Milliseconds()
		if timestamp > maxTimestamp {
			panic("ID 时间位已耗尽")
		}
		if timestamp == g.lastTime && g.sequence == maxSequence {
			time.Sleep(time.Millisecond)
			continue
		}
		if timestamp == g.lastTime {
			g.sequence++
		} else {
			g.sequence = 1
		}
		g.lastTime = timestamp
		orderTime := time.UnixMilli(epoch + timestamp).In(orderLocation)
		return fmt.Sprintf("%s%03d%04d%04d", orderTime.Format("20060102150405"), timestamp%1000, g.nodeID, g.sequence)
	}
}
