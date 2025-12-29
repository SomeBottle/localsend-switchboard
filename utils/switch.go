package utils

// 数据交换相关的工具函数

import (
	"crypto/rand"
	"errors"
	"net"
	"strconv"

	"github.com/somebottle/localsend-switch/constants"
	"github.com/somebottle/localsend-switch/entities"
	switchdata "github.com/somebottle/localsend-switch/generated/switchdata/v1"
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

// SwitchMessageToLocalSendClientInfo 将交换消息转换为 LocalSend 客户端信息实体
func SwitchMessageToLocalSendClientInfo(switchMsg *entities.SwitchMessage) (*entities.LocalSendClientInfo, error) {
	if switchMsg.Payload == nil {
		return nil, errors.New("Switch message does not contain client info")
	}
	discoveryMsg := switchMsg.Payload
	clientInfo := &entities.LocalSendClientInfo{
		Alias:       discoveryMsg.Alias,
		Version:     discoveryMsg.Version,
		DeviceModel: discoveryMsg.DeviceModel,
		DeviceType:  discoveryMsg.DeviceType,
		Fingerprint: discoveryMsg.Fingerprint,
		Port:        uint16(discoveryMsg.Port),
		Protocol:    discoveryMsg.Protocol,
		Download:    discoveryMsg.Download,
	}
	return clientInfo, nil
}

// packLocalSendClientInfoIntoSwitchMessage 将 LocalSend 客户端信息打包进交换消息
//
// nodeId: 节点 ID
// discoverySeq: 发现包序列号
// selfIP: 本机 IP 地址，用于填充 original_addr 字段
func PackLocalSendClientInfoIntoSwitchMessage(clientInfo *entities.LocalSendClientInfo, nodeId string, discoverySeq uint64, selfIP net.IP) *entities.SwitchMessage {
	discoveryMsg := &switchdata.DiscoveryMessage{
		SwitchId:     nodeId,
		DiscoverySeq: discoverySeq,
		DiscoveryTtl: constants.MaxDiscoveryMessageTTL,
		Alias:        clientInfo.Alias,
		Version:      clientInfo.Version,
		DeviceModel:  clientInfo.DeviceModel,
		DeviceType:   clientInfo.DeviceType,
		Fingerprint:  clientInfo.Fingerprint,
		Port:         int32(clientInfo.Port),
		Protocol:     clientInfo.Protocol,
		Download:     clientInfo.Download,
		OriginalAddr: selfIP.String(),
	}
	return &entities.SwitchMessage{
		// SourceAddr 可以不用填，发送时只看 Payload
		Payload: discoveryMsg,
	}
}
