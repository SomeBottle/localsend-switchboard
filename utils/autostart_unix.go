//go:build unix

package utils

import "errors"

// SetAutoStart 在 Unix 系统上设置开机自启
func SetAutoStart(enable bool) error {
	// 尚未实现
	return errors.New("autostart is not supported on this platform, try to use Docker instead.")
}
