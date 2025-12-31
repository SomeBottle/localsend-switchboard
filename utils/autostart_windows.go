//go:build windows

package utils

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// Windows 注册表中用于设置开机自启的键路径
const autostartKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// 值名称
const autostartValueName = "LocalSendSwitchboard"

// SetAutoStart 在 Windows 系统上设置开机自启
//
// enable: 是否启用自启动
func SetAutoStart(enable bool) error {
	// 创建或打开用户注册表项
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartKeyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("Unable to open autostart registry key: %w", err)
	}
	defer key.Close()
	if enable {
		// 要启用自启动，设置值
		exePath, err := GetExactExecutablePath()
		if err != nil {
			return fmt.Errorf("Unable to set autostart, failed to get executable path: %w", err)
		}
		// 设置注册表值
		key.SetStringValue(autostartValueName, exePath)
	} else {
		// 如果要禁用自启且键存在，则删除值
		_, _, err := key.GetStringValue(autostartValueName)
		if err != nil {
			if errors.Is(err, registry.ErrNotExist) {
				// 值不存在，无需删除
				return nil
			}
			return fmt.Errorf("Unable to check autostart registry value: %w", err)
		}
		err = key.DeleteValue(autostartValueName)
		if err != nil {
			return fmt.Errorf("Unable to delete autostart registry value: %w", err)
		}
	}
	return nil
}
