package constants

// 交换机制相关常量

const (
	// 交换消息 ID 缓存的生命周期，单位为秒
	SWITCH_ID_CACHE_LIFETIME = 120 
	// 交换消息 ID 缓存的最大条目数
	SWITCH_ID_CACHE_MAX_ENTRIES = 65536
	// 交换数据等候区大小，即本地停留的发现信息最大条目数，多余的会被丢弃
	SWITCH_LOUNGE_SIZE = 255 * 255
	// 本地客户端信息缓存时间，单位为秒
	LOCAL_CLIENT_INFO_CACHE_LIFETIME = 60
	// 定时广播本地客户端信息的时间间隔，单位为秒
	LOCAL_CLIENT_BROADCAST_INTERVAL = 30
	// 定时探测本地客户端存活的时间间隔，单位为秒
	LOCAL_CLIENT_ALIVE_CHECK_INTERVAL = 15
)
