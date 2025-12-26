package services

// 交换服务核心模块

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/somebottle/localsend-switch/constants"
	"github.com/somebottle/localsend-switch/entities"
	switchdata "github.com/somebottle/localsend-switch/generated/switchdata/v1"
	"google.golang.org/protobuf/proto"
)

// handleTCPConnection 处理并维护单个 TCP 连接
//
// conn: TCP 连接
// dataChan: 传递接收到的交换数据的通道
// tcpConnHub: 维护 TCP 连接的管理器
// sigCtx: 中断信号上下文，用于优雅关闭连接
func handleTCPConnection(conn *net.TCPConn, dataChan chan<- *entities.SwitchMessage, tcpConnHub *TCPConnectionHub, sigCtx context.Context) {
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
			// handleTCP 协程退出，这里也顺带退出
			return
		}
	}()
	// 设置连接的一些传输层属性
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(constants.TCPConnHeartbeatInterval * time.Second)
	// 接收数据
	buf := make([]byte, constants.TCPSocketReadBufferSize)
	for {
		// 设置读取超时，超过心跳时间没有数据就断开连接
		conn.SetReadDeadline(time.Now().Add(constants.TCPConnHeartbeatInterval * time.Second))
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
			if dataLength > constants.TCPSocketReadBufferSize {
				// 数据长度超过缓冲区大小，直接丢弃连接
				return
			}
			// 接下来读取 dataLength 字节的数据
			payload := buf[:dataLength]
			if _, err := io.ReadFull(conn, payload); err != nil {
				// 读取数据失败，可能是连接出错
				return
			}
			// 反序列化数据
			DiscoveryMessage := &switchdata.DiscoveryMessage{}
			if err := proto.Unmarshal(payload, DiscoveryMessage); err != nil {
				// 反序列化失败，可能是数据格式错误，直接丢弃连接
				return
			}
			// 发送数据到通道
			dataChan <- &entities.SwitchMessage{
				SourceAddr: conn.RemoteAddr(),
				Payload:    DiscoveryMessage,
			}
		default:
			// 未知的数据类型，也是直接丢弃连接
			fmt.Printf("Unknown data type received over TCP: 0x%02X, closing connection\n", dataType)
			return
		}
	}
}

// setupTCPServer 通过 TCP 接收来自其他节点的交换数据
//
// servPort: 监听的服务端口
// tcpConnHub: 维护 TCP 连接的管理器
// dataChan: 传递接收到的交换数据的通道
// errChan: 传递错误信息的通道
// sigCtx: 中断信号上下文，用于优雅关闭服务
func setupTCPServer(servPort string, tcpConnHub *TCPConnectionHub, dataChan chan<- *entities.SwitchMessage, errChan chan<- error, sigCtx context.Context) {
	for {
		// 端口转整数
		port, err := strconv.Atoi(servPort)
		if err != nil {
			errChan <- fmt.Errorf("Invalid service port: %v", err)
			return
		}
		exit, err := func() (bool, error) {
			// 启动 TCP 服务
			tcpListener, err := net.ListenTCP("tcp", &net.TCPAddr{
				Port: port,
			})
			if err != nil {
				return true, err
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
			fmt.Printf("TCP Server listening on port %s\n", servPort)
			// 接受连接
			for {
				tcpListener.SetDeadline(time.Now().Add(constants.TCPAcceptTimeout * time.Second))
				conn, err := tcpListener.AcceptTCP()
				if err != nil {
					if sigCtx.Err() != nil {
						// 收到中断信号，优雅退出
						fmt.Printf("TCP Server exiting gracefully...\n")
						return true, nil
					}
					continue
				}
				// 如果超过最大连接数，拒绝连接
				if tcpConnHub.NumConnections() >= constants.MaxTCPConnections {
					fmt.Printf("Maximum TCP connections reached (%d), rejecting new connection from %s\n", constants.MaxTCPConnections, conn.RemoteAddr().String())
					conn.Close()
					continue
				}
				// 添加连接到管理器
				if err := tcpConnHub.AddConnection(conn); err != nil {
					// 添加失败，说明连接已存在
					conn.Close()
					continue
				}
				// 处理连接
				go handleTCPConnection(conn, dataChan, tcpConnHub, sigCtx)
			}
		}()
		if exit {
			// 收到退出信号
			if err != nil {
				errChan <- err
			}
			break
		}

		fmt.Printf("Restarting TCP Server...\nPrevious error: %v\n", err)
		time.Sleep(constants.TCPServerRestartInterval * time.Second)
	}
}

// SetUpSwitchCore 设置并启动交换服务核心模块
func SetUpSwitchCore(peerAddr string, peerPort string, servPort string, sigCtx context.Context, multicastChan <-chan *entities.SwitchMessage, errChan chan<- error) {
	// 通过 TCP 传输的交换数据通道
	switchDataChan := make(chan *entities.SwitchMessage, constants.SwitchDataReceiveChanSize)
	// 维护 TCP 连接的管理器
	var tcpConnHub *TCPConnectionHub = NewTCPConnectionHub()

	// 启动 TCP 服务以接收另一端传输过来的交换数据
	go setupTCPServer(servPort, tcpConnHub, switchDataChan, errChan, sigCtx)
	
}
