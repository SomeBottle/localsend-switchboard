package services

// TCP 核心模块，包括连接处理和维持，TCP 服务启动等

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/somebottle/localsend-switch/configs"
	"github.com/somebottle/localsend-switch/entities"
	switchdata "github.com/somebottle/localsend-switch/generated/switchdata/v1"
	"github.com/somebottle/localsend-switch/utils"
	"google.golang.org/protobuf/proto"
)

// handleTCPConnectionRecv 处理并维护单个 TCP 连接的接收部分
//
// conn: TCP 连接
// recvDataChan: 传递接收到的交换数据的通道
// tcpConnHub: 维护 TCP 连接的管理器
// sigCtx: 中断信号上下文，用于优雅关闭连接
func handleTCPConnectionRecv(conn *net.TCPConn, recvDataChan chan<- *entities.SwitchMessage, tcpConnHub *TCPConnectionHub, sigCtx context.Context) {
	// 用来向中断信号监听协程发送退出信号的管道
	handlerDone := make(chan struct{})
	// 本处理协程终止后的清理
	defer func() {
		close(handlerDone)
		// 从连接管理器中移除连接
		tcpConnHub.RemoveConnection(conn)
		conn.Close()
	}()
	// 监听中断信号
	go func() {
		select {
		case <-sigCtx.Done():
			conn.Close()
		case <-handlerDone:
			// handleTCPConnectionRecv 协程退出，这里也顺带退出
			return
		}
	}()
	// 设置连接的一些传输层属性
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(configs.TCPConnHeartbeatInterval * time.Second)
	// 接收数据
	buf := make([]byte, configs.TCPSocketReadBufferSize)
	// AES 加密工具
	switchDataCipherUtil := utils.GetSwitchDataCipherUtilInstance()
	for {
		// 设置读取超时，超过心跳时间没有数据就断开连接
		conn.SetReadDeadline(time.Now().Add(configs.TCPConnHeartbeatInterval * time.Second))
		// 每组数据传输格式: [ 1 字节的数据类型 | 4 字节的大端数据长度 | 数据 ]

		// 1 字节的数据类型
		//
		// 0x01 - DiscoveryMessage 数据
		// 0x02 - 心跳包
		var dataType byte
		if err := binary.Read(conn, binary.BigEndian, &dataType); err != nil {
			// 读取类型失败，可能是连接出错 / 超时
			return
		}
		switch dataType {
		case 0x02:
			// 心跳包，什么都不做，继续等待下一个数据
			continue
		case 0x01:
			// DiscoveryMessage 数据

			// 4 字节的数据长度
			var dataLength uint32
			if err := binary.Read(conn, binary.BigEndian, &dataLength); err != nil {
				// 读取长度失败，可能是连接出错
				return
			}
			if dataLength > configs.TCPSocketReadBufferSize {
				// 数据长度超过缓冲区大小，直接丢弃连接
				return
			}
			// 接下来读取 dataLength 字节的数据
			payload := buf[:dataLength]
			if _, err := io.ReadFull(conn, payload); err != nil {
				// 读取数据失败，可能是连接出错
				return
			}
			// 解密
			payload, err := switchDataCipherUtil.Decrypt(payload)
			if err != nil {
				// 解密失败，可能数据被篡改，直接丢弃连接
				slog.Debug("Failed to decrypt switch discovery message received over TCP, corrupted or invalid.", "error", err)
				return
			}
			// 反序列化数据
			DiscoveryMessage := &switchdata.DiscoveryMessage{}
			if err := proto.Unmarshal(payload, DiscoveryMessage); err != nil {
				// 反序列化失败，可能是数据格式错误，直接丢弃连接
				slog.Debug("Failed to unmarshal switch discovery message received over TCP, corrupted or invalid.", "error", err)
				return
			}
			// 发送数据到通道
			recvDataChan <- &entities.SwitchMessage{
				SourceAddr: conn.RemoteAddr(),
				Payload:    DiscoveryMessage,
			}
		default:
			// 未知的数据类型，也是直接丢弃连接
			slog.Debug("Unknown data type received over TCP, closing connection", "dataType", dataType)
			return
		}
	}
}

// handleTCPConnectionSend 处理并维护单个 TCP 连接的发送部分
//
// conn: TCP 连接
// sendDataChan: 传递要发送的交换数据的通道
// tcpConnHub: 维护 TCP 连接的管理器
// sigCtx: 中断信号上下文，用于优雅关闭连接
func handleTCPConnectionSend(conn *net.TCPConn, sendDataChan <-chan *entities.SwitchMessage, sigCtx context.Context) {
	// 用于定时发心跳包的定时器
	heartbeatTicker := time.NewTicker(configs.TCPConnHeartbeatSendInterval * time.Second)
	defer heartbeatTicker.Stop()
	// 设置连接的一些传输层属性
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(configs.TCPConnHeartbeatInterval * time.Second)
	// 发送数据
	for {
		select {
		case <-sigCtx.Done():
			// 收到退出信号
			return
		case msg, ok := <-sendDataChan:
			if !ok {
				// 通道关闭，退出
				return
			}
			// 把数据序列化
			payload, err := proto.Marshal(msg.Payload)
			if err != nil {
				// 序列化失败，忽略该数据
				slog.Debug("Failed to marshal switch message for sending over TCP", "message", msg.Payload, "error", err)
				continue
			}
			// 设置写入超时时间
			conn.SetWriteDeadline(time.Now().Add(configs.TCPSocketWriteTimeout * time.Second))

			// 发送数据格式: [ 1 字节的数据类型 | 4 字节的大端数据长度 | 数据 ]

			// 1 字节的数据类型
			var dataType byte = 0x01 // DiscoveryMessage 数据
			if err := binary.Write(conn, binary.BigEndian, dataType); err != nil {
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
					// 连接已关闭，退出协程
					return
				}
				// 发送类型失败，可能是连接出错
				slog.Debug("Failed to send data type over TCP connection", "remoteAddr", conn.RemoteAddr().String(), "error", err)
				continue
			}
			// 加密数据
			payload, err = utils.GetSwitchDataCipherUtilInstance().Encrypt(payload)
			if err != nil {
				// 加密失败，忽略该数据
				slog.Error("Failed to encrypt switch message for sending over TCP", "message", msg.Payload, "error", err)
				continue
			}
			// 4 字节的大端数据长度
			dataLength := uint32(len(payload))
			if err := binary.Write(conn, binary.BigEndian, dataLength); err != nil {
				// 发送长度失败，可能是连接出错
				slog.Debug("Failed to send data length over TCP connection", "remoteAddr", conn.RemoteAddr().String(), "error", err)
				continue
			}
			// 发送数据
			if err := utils.WriteAllBytes(conn, payload); err != nil {
				// 发送数据失败，可能是连接出错
				slog.Debug("Failed to send data over TCP connection", "remoteAddr", conn.RemoteAddr().String(), "error", err)
				continue
			}
		case <-heartbeatTicker.C:
			// 发送心跳包
			conn.SetWriteDeadline(time.Now().Add(configs.TCPSocketWriteTimeout * time.Second))
			var heartbeatByte byte = 0x02
			if err := binary.Write(conn, binary.BigEndian, heartbeatByte); err != nil {
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
					// 连接已关闭，退出协程
					return
				}
				// 发送心跳包失败，可能是连接出错
				slog.Debug("Failed to send heartbeat over TCP connection", "remoteAddr", conn.RemoteAddr().String(), "error", err)
				continue
			}
		}
	}
}

// handleTCPConnection 处理并维护单个 TCP 连接
//
// conn: TCP 连接
// sendDataChan: 传递要发送的交换数据的通道
// recvDataChan: 传递接收到的交换数据的通道
// tcpConnHub: 维护 TCP 连接的管理器
// sigCtx: 中断信号上下文，用于优雅关闭连接
func handleTCPConnection(conn *net.TCPConn, sendDataChan <-chan *entities.SwitchMessage, recvDataChan chan<- *entities.SwitchMessage, tcpConnHub *TCPConnectionHub, sigCtx context.Context) {
	// 启动接收协程
	go handleTCPConnectionRecv(conn, recvDataChan, tcpConnHub, sigCtx)
	// 启动发送协程
	handleTCPConnectionSend(conn, sendDataChan, sigCtx)
}

// connectPeer 连接到另一个 switch 节点并维护该连接
//
// peerAddr: 另一个 switch 节点的地址
// peerPort: 另一个 switch 节点的端口
// tcpConnHub: 维护 TCP 连接的管理器
// switchDataChan: 传递交换数据的通道
// errChan: 错误通道，用于传递运行时错误
// sigCtx: 中断信号上下文，用于优雅关闭协程
func connectPeer(peerAddr string, peerPort string, tcpConnHub *TCPConnectionHub, switchDataChan chan *entities.SwitchMessage, errChan chan<- error, sigCtx context.Context) {
	// 没有配置 peerAddr 或 peerPort 则不启动转发协程
	if peerAddr == "" || peerPort == "" {
		slog.Info("Peer address or port not provided, switch forwarder will not be started")
		return
	}
	// 建立 TCP 连接重试计数器
	var retryCount int
	port, err := strconv.Atoi(peerPort)
	if err != nil {
		errChan <- fmt.Errorf("Invalid peer port: %v", err)
		return
	}
	for {
		exit, err := func() (bool, error) {
			// 和另一个 switch 端建立 TCP 连接
			conn, tcpErr := net.DialTCP("tcp", nil, &net.TCPAddr{
				IP:   net.ParseIP(peerAddr),
				Port: port,
			})
			if tcpErr != nil {
				return false, tcpErr
			}
			// 用于通知中断监听协程退出的管道
			connDone := make(chan struct{})
			// 资源释放
			defer func() {
				close(connDone)
				conn.Close()
			}()
			// 中断信号监听协程
			go func() {
				for {
					select {
					case <-sigCtx.Done():
						// 接到退出信号
						conn.Close()
						return
					case <-connDone:
						// 退出协程
						return
					}
				}
			}()
			// 成功建立连接，重试计数重置
			retryCount = 0
			// 添加连接到管理器
			sendChan, err := tcpConnHub.AddConnection(conn)
			if err != nil {
				// 添加失败，说明连接已存在或者超过最大连接数，这种情况下退出
				slog.Warn("Failed to create TCP connection to peer switch", "peerAddr", peerAddr, "peerPort", peerPort, "error", err)
				return true, nil
			}
			slog.Info("Established TCP connection to peer switch", "peerAddr", peerAddr, "peerPort", peerPort)
			// 处理并维持连接
			handleTCPConnection(conn, sendChan, switchDataChan, tcpConnHub, sigCtx)
			if sigCtx.Err() != nil {
				// 收到退出信号，优雅退出
				slog.Debug("Peer connection exiting gracefully", "peerAddr", peerAddr, "peerPort", peerPort)
				return true, nil
			}
			// 连接意外断开，继续重试
			return false, nil
		}()
		if exit {
			// 收到退出信号，终止协程
			if err != nil {
				errChan <- err
			}
			return
		}
		// 意外退出，继续重试
		retryCount++
		if configs.GetSwitchPeerConnectMaxRetries() < 0 {
			// 如果为负数则无限重试
			slog.Info("Retrying to connect to peer switch", "peerAddr", peerAddr, "peerPort", peerPort, "interval", configs.SwitchPeerConnectRetryInterval, "retryCount", retryCount, "maxRetries", "unlimited")
		} else {
			if retryCount > configs.GetSwitchPeerConnectMaxRetries() {
				// 重试次数过多
				errChan <- fmt.Errorf("Exceeded maximum retries (%d) to connect to peer switch at %s:%s", configs.GetSwitchPeerConnectMaxRetries(), peerAddr, peerPort)
				return
			}
			slog.Info("Retrying to connect to peer switch", "peerAddr", peerAddr, "peerPort", peerPort, "interval", configs.SwitchPeerConnectRetryInterval, "retryCount", retryCount, "maxRetries", configs.GetSwitchPeerConnectMaxRetries())
		}
		time.Sleep(configs.SwitchPeerConnectRetryInterval * time.Second)
	}
}

// setUpTCPServer 通过 TCP 接收来自其他节点的交换数据
//
// servPort: 监听的服务端口
// tcpConnHub: 维护 TCP 连接的管理器
// dataChan: 传递接收到的交换数据的通道
// errChan: 传递错误信息的通道
// sigCtx: 中断信号上下文，用于优雅关闭服务
func setUpTCPServer(servPort string, tcpConnHub *TCPConnectionHub, dataChan chan<- *entities.SwitchMessage, errChan chan<- error, sigCtx context.Context) {
	if servPort == "" {
		// 未配置服务端口，不启动 TCP 服务
		slog.Info("Service port not provided, TCP server will not be started")
		return
	}
	for {
		// 端口转整数
		port, err := strconv.Atoi(servPort)
		if err != nil {
			errChan <- fmt.Errorf("Invalid service port: %v", err)
			return
		}
		exit, err := func() (bool, error) {
			// 启动 TCP 服务
			tcpListener, tcpErr := net.ListenTCP("tcp", &net.TCPAddr{
				Port: port,
			})
			if tcpErr != nil {
				return true, tcpErr
			}
			// 用于通知中断监听协程退出的管道
			listenerDone := make(chan struct{})
			// 资源释放
			defer func() {
				close(listenerDone)
				tcpListener.Close()
			}()
			// 中断信号监听协程
			go func() {
				for {
					select {
					case <-sigCtx.Done():
						// 接到退出信号，关闭监听器，终止服务
						tcpListener.Close()
						return
					case <-listenerDone:
						// 退出协程
						return
					}
				}
			}()
			slog.Info("TCP Server listening on port", "port", servPort)
			// 接受连接
			for {
				tcpListener.SetDeadline(time.Now().Add(configs.TCPAcceptTimeout * time.Second))
				conn, err := tcpListener.AcceptTCP()
				if err != nil {
					if sigCtx.Err() != nil {
						// 收到中断信号，优雅退出
						slog.Debug("TCP Server exiting gracefully")
						return true, nil
					}
					continue
				}
				// 添加连接到管理器
				sendChan, err := tcpConnHub.AddConnection(conn)
				if err != nil {
					// 添加失败，说明连接已存在或者超过最大连接数
					slog.Warn("Failed to add TCP connection", "remoteAddr", conn.RemoteAddr().String(), "error", err)
					conn.Close()
					continue
				}
				// 处理连接
				go handleTCPConnection(conn, sendChan, dataChan, tcpConnHub, sigCtx)
				slog.Info("Accepted TCP connection", "remoteAddr", conn.RemoteAddr().String())
			}
		}()
		if exit {
			// 收到退出信号
			if err != nil {
				errChan <- err
			}
			break
		}

		slog.Info("Restarting TCP Server", "previousError", err)
		time.Sleep(configs.TCPServerRestartInterval * time.Second)
	}
}
