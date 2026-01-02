package utils

// 日志相关工具

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/somebottle/localsend-switch/entities"
)

// rotatedLogFilePattern 用于匹配轮转日志文件名
const rotatedLogFilePattern = `^(.+?)_rotated\.(\d+)\.log$`

// rotatedLogFileFormat 为轮转日志文件名格式
const rotatedLogFileFormat = "%s_rotated.%d.log"

// rotatedLogFileRegex 是用于匹配轮转日志文件名的正则表达式
var rotatedLogFileRegex = regexp.MustCompile(rotatedLogFilePattern)

// LogWriter 是简单的日志写入器，支持日志轮转
type LogWriter struct {
	filePath          string
	fileName          string
	fileDir           string
	maxSize           int64 // 以字节为单位的最大文件大小
	maxHistoricalLogs int   // 最大历史日志文件数量
	file              *os.File
	closed            bool
}

// NewLogWriter 创建一个新的 LogWriter 实例
func NewLogWriter(filePath string, maxSize int64, maxHistoricalLogs int) (*LogWriter, error) {
	fileDir := filepath.Dir(filePath)
	// 创建必要目录
	err := os.MkdirAll(fileDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("Failed to create log directory: %w", err)
	}
	fileName := filepath.Base(filePath)
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("Failed to open log file: %w", err)
	}
	return &LogWriter{
		filePath:          filePath,
		fileName:          fileName,
		fileDir:           fileDir,
		maxSize:           maxSize,
		maxHistoricalLogs: maxHistoricalLogs,
		file:              file,
		closed:            false,
	}, nil
}

// rotateLogs 执行日志轮转，将当前日志文件重命名为轮转文件，并管理历史日志文件数量
func (lw *LogWriter) rotateLogs() error {
	// 关闭当前日志文件
	lw.file.Close()
	// 找到目录下有多少历史日志文件
	files, err := os.ReadDir(lw.fileDir)
	if err != nil {
		return fmt.Errorf("Failed to read log directory: %w", err)
	}
	logFileNameAndIds := []entities.RotatedLogFileName{}
	// 收集所有轮转日志文件名
	for _, file := range files {
		if submatches := rotatedLogFileRegex.FindStringSubmatch(file.Name()); len(submatches) == 3 {
			logId, parseErr := strconv.ParseInt(submatches[2], 10, 32)
			if parseErr != nil {
				continue
			}
			logFileNameAndIds = append(logFileNameAndIds, entities.RotatedLogFileName{
				FullName: file.Name(),
				LogId:    int(logId),
				BaseName: submatches[1],
			})
		}
	}
	// 按 ID 升序排序
	sort.Slice(logFileNameAndIds, func(i, j int) bool {
		return logFileNameAndIds[i].LogId < logFileNameAndIds[j].LogId
	})
	// 如果超过最大历史日志文件数量，删除最旧的文件，并依次重命名其他文件
	numHistoricalLogs := len(logFileNameAndIds)
	if numHistoricalLogs >= lw.maxHistoricalLogs {
		// +1 是算上了当前的日志文件，总共多了这么多历史日志文件
		numOverflow := numHistoricalLogs + 1 - lw.maxHistoricalLogs
		for i := numHistoricalLogs - numOverflow; i < numHistoricalLogs; i++ {
			toDeleteFilePath := filepath.Join(lw.fileDir, logFileNameAndIds[i].FullName)
			err := os.Remove(toDeleteFilePath)
			if err != nil {
				return fmt.Errorf("Failed to delete old log file '%s': %w", toDeleteFilePath, err)
			}
		}
		logFileNameAndIds = logFileNameAndIds[:numHistoricalLogs-numOverflow]
	}
	// 依次重命名现有的历史日志文件，ID 加 1
	for i := len(logFileNameAndIds) - 1; i >= 0; i-- {
		oldFilePath := filepath.Join(lw.fileDir, logFileNameAndIds[i].FullName)
		newLogId := logFileNameAndIds[i].LogId + 1
		newFileName := fmt.Sprintf(rotatedLogFileFormat, logFileNameAndIds[i].BaseName, newLogId)
		newFilePath := filepath.Join(lw.fileDir, newFileName)
		err := os.Rename(oldFilePath, newFilePath)
		if err != nil {
			return fmt.Errorf("Failed to rename log file '%s' to '%s': %w", oldFilePath, newFilePath, err)
		}
	}
	// 重命名当前日志文件为轮转文件，ID 为 1
	rotatedFileName := fmt.Sprintf(rotatedLogFileFormat, GetBaseNameWithoutExt(lw.fileName), 1)
	rotatedFilePath := filepath.Join(lw.fileDir, rotatedFileName)
	err = os.Rename(lw.filePath, rotatedFilePath)
	if err != nil {
		return fmt.Errorf("Failed to rotate current log file to '%s': %w", rotatedFilePath, err)
	}
	// 重新打开一个新的日志文件
	lw.file, err = os.OpenFile(lw.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("Failed to open new log file: %w", err)
	}
	return nil
}

// Write 写入日志数据，并在达到最大文件大小时进行轮转，实现了 io.Writer 接口
func (lw *LogWriter) Write(p []byte) (n int, err error) {
	if lw.closed {
		return 0, errors.New("LogWriter is closed")
	}
	if lw.file == nil {
		return 0, errors.New("Log file is not open")
	}
	// 检查当前文件大小
	fileInfo, err := lw.file.Stat()
	if err != nil {
		return 0, fmt.Errorf("Failed to get log file info: %w", err)
	}
	currFileSize := fileInfo.Size()
	// 如果写入后超过最大大小，则进行轮转
	if currFileSize+int64(len(p)) > lw.maxSize {
		err := lw.rotateLogs()
		if err != nil {
			return 0, fmt.Errorf("Failed to rotate logs: %w", err)
		}
	}
	// 写入数据
	n, err = lw.file.Write(p)
	return n, err
}

// Close 关闭日志写入器以及相关文件资源
func (lw *LogWriter) Close() error {
	if lw.file == nil {
		return errors.New("Log file is not open")
	}
	if lw.closed {
		return nil
	}
	if lw.file != nil {
		err := lw.file.Close()
		if err != nil {
			return err
		}
	}
	lw.closed = true
	return nil
}
