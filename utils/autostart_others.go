//go:build !linux && !windows

package utils

import "errors"

// SetAutoStart 在其他系统上设置开机自启
//
// 由于尚未实现，调用该函数会返回错误
func SetAutoStart(enable bool) error {
	// 尚未实现
	return errors.New("autostart is not implemented on this platform, try to use Docker or something else.")
}
