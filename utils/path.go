package utils

// 一些路径相关的工具函数

import (
	"os"
	"path/filepath"
)

// GetExactExecutablePath 获取当前进程可执行文件的真实绝对路径
func GetExactExecutablePath() (string, error) {
	exeAbsPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	realExePath, err := filepath.EvalSymlinks(exeAbsPath)
	if err != nil {
		return "", err
	}
	return realExePath, nil
}

// GetExactExecutableDir 获取当前进程可执行文件所在目录的真实绝对路径
func GetExactExecutableDir() (string, error) {
	exePath, err := GetExactExecutablePath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exePath), nil
}

// GetBaseNameWithoutExt 获取路径的文件名（不含扩展名）
func GetBaseNameWithoutExt(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return base[:len(base)-len(ext)]
}
