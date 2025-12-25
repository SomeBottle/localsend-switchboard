package entities

// 网络处理相关实体

import (
	"net"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	switchdata "github.com/somebottle/localsend-switch/generated/switchdata/v1"
)

// PacketConn 封装了 IPv4 和 IPv6 的数据包连接，包括有 ReadFrom 和 Close 方法
type PacketConn struct {
	IPv4Conn *ipv4.PacketConn
	IPv6Conn *ipv6.PacketConn
}

// ReadFrom 从连接中读取数据包
func (pc *PacketConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	if pc.IPv4Conn != nil {
		n, _, addr, err := pc.IPv4Conn.ReadFrom(b)
		return n, addr, err
	}
	if pc.IPv6Conn != nil {
		n, _, addr, err := pc.IPv6Conn.ReadFrom(b)
		return n, addr, err
	}
	return 0, nil, nil
}

// SetReadDeadline 设置读取超时时刻
func (pc *PacketConn) SetReadDeadline(t time.Time) error {
	if pc.IPv4Conn != nil {
		if err := pc.IPv4Conn.SetReadDeadline(t); err != nil {
			return err
		}
	}
	if pc.IPv6Conn != nil {
		if err := pc.IPv6Conn.SetReadDeadline(t); err != nil {
			return err
		}
	}
	return nil
}

// Close 关闭连接
func (pc *PacketConn) Close() error {
	if pc.IPv4Conn != nil {
		if err := pc.IPv4Conn.Close(); err != nil {
			return err
		}
	}
	if pc.IPv6Conn != nil {
		if err := pc.IPv6Conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

// SwitchMessage 包装了连接要递交的交换数据
type SwitchMessage struct {
	SourceAddr string
	Payload   *switchdata.ClientInfo
}