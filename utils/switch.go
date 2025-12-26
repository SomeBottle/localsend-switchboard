package utils

// 数据交换相关的工具函数

import (
	"crypto/rand"
	"strconv"
	"github.com/somebottle/localsend-switch/entities"
)

const ID_LETTERS = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateRandomSwitchID 生成一个随机的 16 字节 Switch ID，用于标识交换节点
func GenerateRandomSwitchID() string {
	randomBytes := make([]byte, 16)
	_, _ = rand.Read(randomBytes)
	for i, b := range randomBytes {
		randomBytes[i] = ID_LETTERS[int(b)%len(ID_LETTERS)]
	}
	return string(randomBytes)
}

// GetDiscoveryId 从交换消息中获取唯一的发现包 ID，格式为 SwitchId_DiscoverySeq
func GetDiscoveryId(switchMsg *entities.SwitchMessage) string {
	var discoveryId string = switchMsg.Payload.SwitchId + "_" + strconv.FormatUint(switchMsg.Payload.DiscoverySeq, 10)
	return discoveryId
}