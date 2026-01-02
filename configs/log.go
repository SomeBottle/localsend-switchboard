package configs

// 日志相关默认配置

var (
	// LogFilePath 为默认日志文件路径
	logFilePath string = "localsend-switch-logs/latest.log"
	// LogMaxSizeBytes 日志文件的最大大小，超过该大小会进行轮转，单位为字节
	logMaxSizeBytes int64 = 5 * 1024 * 1024 // 5 MB
	// LogMaxHistoricalFiles 最大历史日志文件数量，超过该数量会删除最旧的日志文件
	logMaxHistoricalFiles int = 5
)

// GetLogFilePath 获取默认日志文件路径
func GetLogFilePath() string {
	return logFilePath
}

// SetLogFilePath 设置默认日志文件路径
func SetLogFilePath(path string) {
	logFilePath = path
}

// GetLogMaxSizeBytes 获取日志文件的最大大小，单位为字节
func GetLogMaxSizeBytes() int64 {
	return logMaxSizeBytes
}

// SetLogMaxSizeBytes 设置日志文件的最大大小，单位为字节
func SetLogMaxSizeBytes(size int64) {
	logMaxSizeBytes = size
}

// GetLogMaxHistoricalFiles 获取最大历史日志文件数量
func GetLogMaxHistoricalFiles() int {
	return logMaxHistoricalFiles
}

// SetLogMaxHistoricalFiles 设置最大历史日志文件数量
func SetLogMaxHistoricalFiles(count int) {
	logMaxHistoricalFiles = count
}
