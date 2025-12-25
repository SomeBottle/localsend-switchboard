package services

// TCP 连接管理模块

import (
	"errors"
	"net"
)

type TCPConnectionHub struct {
	conns map[string]*net.TCPConn
}

// NewTCPConnectionHub 创建一个新的 TCP 连接管理器
func NewTCPConnectionHub() *TCPConnectionHub {
	return &TCPConnectionHub{
		conns: make(map[string]*net.TCPConn),
	}
}

// AddConnection 添加一个新的 TCP 连接到管理器
func (hub *TCPConnectionHub) AddConnection(conn *net.TCPConn) error {
	// 使用连接发起地址 (含有端口) 作为键 (标记客户端)
	remoteAddrStr := conn.RemoteAddr().String()
	if _, exists := hub.conns[remoteAddrStr]; exists {
		return errors.New("Connection already exists")
	}
	hub.conns[remoteAddrStr] = conn
	return nil
}

// RemoveConnection 从管理器中移除一个 TCP 连接
func (hub *TCPConnectionHub) RemoveConnection(conn *net.TCPConn) { 
	remoteAddrStr := conn.RemoteAddr().String()
	delete(hub.conns, remoteAddrStr)
}

// NumConnections 返回当前管理的连接数
func (hub *TCPConnectionHub) NumConnections() int {
	return len(hub.conns)
}