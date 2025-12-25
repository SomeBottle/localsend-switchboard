package constants

// 网络处理相关常量
const (
	// LocalSend 默认的 IPv4 组播地址
	LocalSendDefaultMulticastIPv4 = "224.0.0.167"
	// LocalSend 默认的组播端口
	LocalSendDefaultMulticastPort = "53317"
	// 组播数据读取时字节缓冲区大小
	MulticastReadBufferSize = 65536 // 64 KiB
	// 组播数据读取超时时间
	MulticastReadTimeout = 15 // 秒
	// 重试监听组播的间隔时间
	MulticastListenRetryInterval = 3 // 秒
	// TCP 最大连接数
	MaxTCPConnections = 255 * 255
	// TCP 连接等待超时时间
	TCPAcceptTimeout = 30 // 秒
	// TCP 连接心跳间隔时间
	TCPConnHeartbeatInterval = 15 // 秒
	// TCP 服务重启间隔时间
	TCPServerRestartInterval = 3 // 秒
	// 读取 TCP 数据时字节缓冲区大小
	TCPSocketReadBufferSize = 1024 * 1024 // 1 MiB
	// 接收交换数据的缓冲区大小 (通道)
	SwitchDataReceiveChanSize = 128
)
