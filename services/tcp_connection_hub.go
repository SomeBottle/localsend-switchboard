package services

// TCP 连接管理模块

import (
	"errors"
	"net"
	"sync"

	"github.com/somebottle/localsend-switch/constants"
	"github.com/somebottle/localsend-switch/entities"
)

// ConnWithChan 包含 TCP 连接及其发送通道
type ConnWithChan struct {
	Conn     *net.TCPConn
	SendChan chan *entities.SwitchMessage
}

// TCPConnectionHub 管理所有 TCP 连接
type TCPConnectionHub struct {
	// 控制对 conns 的并发访问
	mutex sync.Mutex
	conns map[string]ConnWithChan
}

// NewTCPConnectionHub 创建一个新的 TCP 连接管理器
func NewTCPConnectionHub() *TCPConnectionHub {
	return &TCPConnectionHub{
		conns: make(map[string]ConnWithChan),
	}
}

// AddConnection 添加一个新的 TCP 连接到管理器，并创建其发送通道
func (hub *TCPConnectionHub) AddConnection(conn *net.TCPConn) (<-chan *entities.SwitchMessage, error) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	// 使用连接发起地址 (含有端口) 作为键 (标记客户端)
	remoteAddrStr := conn.RemoteAddr().String()
	if _, exists := hub.conns[remoteAddrStr]; exists {
		return nil, errors.New("Connection already exists")
	}
	// 另外检查连接数是否超过限制
	if len(hub.conns) >= constants.MaxTCPConnections {
		return nil, errors.New("Maximum TCP connections reached, ignoring new connection")
	}
	// 创建发送通道
	sendChan := make(chan *entities.SwitchMessage, constants.TCPSocketSendChanSize)
	hub.conns[remoteAddrStr] = ConnWithChan{
		Conn:     conn,
		SendChan: sendChan,
	}
	return sendChan, nil
}

// RemoveConnection 从管理器中移除一个 TCP 连接
func (hub *TCPConnectionHub) RemoveConnection(conn *net.TCPConn) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	remoteAddrStr := conn.RemoteAddr().String()
	if cwc, exists := hub.conns[remoteAddrStr]; exists {
		// 关闭发送通道
		close(cwc.SendChan)
		cwc.Conn.Close()
		delete(hub.conns, remoteAddrStr)
	}
}

// NumConnections 返回当前管理的连接数
func (hub *TCPConnectionHub) NumConnections() int {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	return len(hub.conns)
}

// GetConnections 返回除了来源为 remoteAddr 以外所有 TCP 连接构成的切片
//
// remoteAddr: 要排除的远端连接地址
func (hub *TCPConnectionHub) GetConnectionsExcept(remoteAddr net.Addr) []ConnWithChan {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	cwcSlice := make([]ConnWithChan, 0, len(hub.conns))
	for addrStr, cwc := range hub.conns {
		if addrStr != remoteAddr.String() {
			cwcSlice = append(cwcSlice, cwc)
		}
	}
	return cwcSlice
}

// Close 关闭所有管理的 TCP 连接
func (hub *TCPConnectionHub) Close() {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	for _, cwc := range hub.conns {
		// 连接关闭后，连接 handler 会自动从管理器中移除该连接
		cwc.Conn.Close()
	}
}
