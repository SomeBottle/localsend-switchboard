//go:build unix
package utils

import (
	"context"
	"net"
	"syscall"
)

// UNIX 系统下的网络相关工具函数

// ListenPacketWithREUSEADDR 创建一个启用套接字 REUSEADDR (复用端口 / 地址) 选项的 PacketConn
//
// 这会允许端口未释放时绑定该端口，适用于组播监听
//
// network: 网络类型 (如 "udp4" 或 "udp6")
// address: 监听地址 (如 ":53317" 或 "[::]:53317")
func ListenPacketWithREUSEADDR(network string, address string) (net.PacketConn, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var controlErr error
			c.Control(func(fd uintptr) {
				controlErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
			return controlErr
		},
	}
	return lc.ListenPacket(context.Background(), network, address)
}
