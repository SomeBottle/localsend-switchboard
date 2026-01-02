package entities

// 日志相关实体定义

type RotatedLogFileName struct {
	BaseName string // 基础文件名，不含轮转部分和扩展名
	LogId    int // 轮转日志文件的 ID
	FullName string // 完整的轮转日志文件名
}