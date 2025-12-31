package configs

// 网络处理相关常量
const (
	// LocalSend 默认的 IPv4 组播地址
	LocalSendDefaultMulticastIPv4 = "224.0.0.167"
	// LocalSend 默认的组播 /HTTP 端口
	LocalSendDefaultPort = "53317"
	// 组播数据读取时字节缓冲区大小
	MulticastReadBufferSize = 65536 // 64 KiB
	// 组播数据读取超时时间
	MulticastReadTimeout = 15 // 秒
	// 组播数据通道缓冲区大小
	MulticastChanSize = 128
	// 重试监听组播的间隔时间
	MulticastListenRetryInterval = 3 // 秒
	// TCP 最大连接数
	MaxTCPConnections = 255 * 255
	// TCP 连接等待超时时间
	TCPAcceptTimeout = 30 // 秒
	// TCP 连接心跳间隔时间
	TCPConnHeartbeatInterval = 15 // 秒
	// TCP 连接心跳发送间隔时间
	TCPConnHeartbeatSendInterval = 8 // 秒
	// TCP 服务重启间隔时间
	TCPServerRestartInterval = 3 // 秒
	// 读取 TCP 数据时字节缓冲区大小
	TCPSocketReadBufferSize = 1024 * 1024 // 1 MiB
	// 接收交换数据的缓冲区大小 (通道)
	SwitchDataReceiveChanSize = 128
	// 发现信息最大跳数
	MaxDiscoveryMessageTTL = 255
	// 和对端 switch 建立 TCP 连接的重试间隔
	SwitchPeerConnectRetryInterval = 3 // 秒
	// 和对端 switch 建立 TCP 连接的最大重试次数
	SwitchPeerConnectMaxRetries = 10
	// TCP 发送通道缓冲区大小
	TCPSocketSendChanSize = 32
	// 写入 TCP 数据的超时时间
	TCPSocketWriteTimeout = 3 // 秒
	// HTTP 请求超时时间
	HTTPRequestTimeout = 2 // 秒
	// HTTP 响应体最大读取字节数
	HTTPResponseBodyMaxSize = 1 * 1024 * 1024 // 1 MiB
	// HTTP 客户端 Worker 数量
	HTTPClientWorkerCount = 8
)
