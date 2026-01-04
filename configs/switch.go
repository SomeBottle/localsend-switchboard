package configs

// 交换机制相关常量

const (
	// 交换消息 ID 缓存的生命周期，单位为秒
	SwitchIDCacheLifetime = 300
	// 交换消息 ID 缓存的最大条目数
	SwitchIDCacheMaxEntries = 65536
	// 交换数据等候区大小，即本地停留的发现信息最大条目数，多余的会被丢弃
	SwitchLoungeSize = 255 * 255
)

var (
	// 定时广播本地客户端信息的时间间隔，单位为秒
	localClientBroadcastInterval = 15
	// 定时探测本地客户端存活的时间间隔，单位为秒
	localClientAliveCheckInterval = 10
	// 本地客户端信息缓存时间，单位为秒
	localClientInfoCacheLifetime = 60
)

// SetLocalClientBroadcastInterval 设置定时广播本地客户端信息的时间间隔，单位为秒
func SetLocalClientBroadcastInterval(seconds int) {
	localClientBroadcastInterval = seconds
}

// GetLocalClientBroadcastInterval 获取定时广播本地客户端信息的时间间隔，单位为秒
func GetLocalClientBroadcastInterval() int {
	return localClientBroadcastInterval
}

// SetLocalClientAliveCheckInterval 设置定时探测本地客户端存活的时间间隔，单位为秒
func SetLocalClientAliveCheckInterval(seconds int) {
	localClientAliveCheckInterval = seconds
	// 保证本地客户端信息缓存时间比存活检查间隔时间至少长 10 秒
	localClientInfoCacheLifetime = max(localClientInfoCacheLifetime, seconds+10)
}

// GetLocalClientAliveCheckInterval 获取定时探测本地客户端存活的时间间隔，单位为秒
func GetLocalClientAliveCheckInterval() int {
	return localClientAliveCheckInterval
}

// GetLocalClientInfoCacheLifetime 获取本地客户端信息缓存时间，单位为秒
func GetLocalClientInfoCacheLifetime() int {
	return localClientInfoCacheLifetime
}