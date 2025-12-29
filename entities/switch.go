package entities

import (
	"net"

	switchdata "github.com/somebottle/localsend-switch/generated/switchdata/v1"
)

// 发现 / 数据交换相关实体

// SwitchMessage 包装了连接要递交的交换数据
type SwitchMessage struct {
	// 数据发送来源地址，可能是中间节点 IP，不一定是发送信息发出的原始地址
	SourceAddr net.Addr
	Payload    *switchdata.DiscoveryMessage
}

// LocalSendClientInfo 存储 LocalSend 客户端信息
//
// 文档: https://github.com/localsend/protocol?tab=readme-ov-file#31-multicast-udp-default
type LocalSendClientInfo struct {
	// 本地客户端别名
	Alias string `json:"alias"`
	// 客户端版本
	Version string `json:"version"`
	// 设备型号
	DeviceModel string `json:"deviceModel"`
	// 设备类型
	DeviceType string `json:"deviceType"`
	// 客户端指纹
	Fingerprint string `json:"fingerprint"`
	// 本地客户端监听的端口
	Port uint16 `json:"port"`
	// 协议 (http / https)
	Protocol string `json:"protocol"`
	// 是否支持下载
	Download bool `json:"download"`
}
