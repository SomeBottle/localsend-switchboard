//go:build linux

package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// entryFormat 是条目的模板
const entryFormat = `[Desktop Entry]
Type=Application
Name=LocalSend Switch
Exec=%s
X-GNOME-Autostart-enabled=true
NoDisplay=true
Comment=Auto start LocalSend Switch on login
Terminal=false
`

// entryFileName  是条目文件名
const entryFileName = "localsend-switch.desktop"

// SetAutoStart 在 Linux 系统上设置开机自启
//
// 根据 XDG Autostart 规范实现, 文档: https://specifications.freedesktop.org/desktop-entry/latest/recognized-keys.html
//
// enable: 是否启用自启动
func SetAutoStart(enable bool) error {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("Unable to get user config directory: %w", err)
	}
	autostartDir := filepath.Join(userConfigDir, "autostart")
	desktopEntryPath := filepath.Join(autostartDir, entryFileName)
	if enable {
		// 创建 autostart 目录（如果不存在）
		if err := os.MkdirAll(autostartDir, 0755); err != nil {
			return fmt.Errorf("Unable to create autostart directory: %w", err)
		}
		// 创建或覆盖桌面条目文件
		exePath, err := GetExactExecutablePath()
		if err != nil {
			return fmt.Errorf("Unable to get executable path: %w", err)
		}
		entryContent := fmt.Sprintf(entryFormat, exePath)
		if err := os.WriteFile(desktopEntryPath, []byte(entryContent), 0644); err != nil {
			return fmt.Errorf("Unable to write autostart desktop entry: %w", err)
		}
	} else {
		// 删除桌面条目文件（如果存在）
		if _, err := os.Stat(desktopEntryPath); err == nil {
			if err := os.Remove(desktopEntryPath); err != nil {
				return fmt.Errorf("Unable to remove autostart desktop entry: %w", err)
			}
		}
	}
	return nil
}
