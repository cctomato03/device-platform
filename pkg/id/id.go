// Package id 雪花 ID 生成器（int64，JSON 序列化为字符串避免前端精度丢失）。
package id

import (
	"strconv"
	"sync"

	"github.com/bwmarrin/snowflake"
)

var (
	node *snowflake.Node
	once sync.Once
)

// Init 初始化，nodeID 取值 0~1023（多实例部署时需保证唯一）。
func Init(nodeID int64) error {
	var err error
	once.Do(func() {
		node, err = snowflake.NewNode(nodeID % 1024)
	})
	return err
}

// Next 生成下一个 ID。
func Next() uint64 {
	if node == nil {
		_ = Init(1)
	}
	return uint64(node.Generate().Int64())
}

// NewString 生成 ID 字符串（用于对象键、文件名等）。
func NewString() string {
	return strconv.FormatUint(Next(), 10)
}
