package services

// 交换服务核心模块

import (
	"context"
	"fmt"

	"github.com/somebottle/localsend-switch/constants"
	"github.com/somebottle/localsend-switch/entities"
	"github.com/somebottle/localsend-switch/utils"
)

// setUpForwarder 启动交换数据转发器
// 
// SwitchLounge: 交换数据等候室
func setUpForwarder(SwitchLounge *SwitchLounge, localClientLounge *LocalClientLounge, tcpConnHub *TCPConnectionHub, errChan chan<- error, sigCtx context.Context) {
	for{
		select {
		case <-sigCtx.Done():
			// 收到退出信号
			return
		case switchMsg, ok := <-SwitchLounge.Read():
			if !ok {
				// 等候室关闭，退出
				return
			}
			// 对于每个交换信息，转发给所有连接的节点 (除开其来源节点)
			for _,cwc := range tcpConnHub.GetConnectionsExcept(switchMsg.SourceAddr) {
				// 把交换信息发送到对应的发送通道
				cwc.SendChan<-switchMsg
			}
			// TODO: 新建另外一个 HTTP 请求发送协程，通过通道发送要发起的 HTTP 请求.
		}
	}
}


// SetUpSwitchCore 设置并启动交换服务核心模块
func SetUpSwitchCore(peerAddr string, peerPort string, servPort string, sigCtx context.Context, multicastChan <-chan *entities.SwitchMessage, errChan chan<- error) {
	// 通过 TCP 传输的交换数据通道
	switchDataChan := make(chan *entities.SwitchMessage, constants.SwitchDataReceiveChanSize)
	// 维护 TCP 连接的管理器
	var tcpConnHub *TCPConnectionHub = NewTCPConnectionHub()
	// 维护待转发交换信息的等候室
	var switchLounge *SwitchLounge = NewSwitchLounge()
	// 维护本地客户端信息的等候室
	var localClientLounge *LocalClientLounge = NewLocalClientLounge()
	// 清理
	defer func() {
		localClientLounge.Close()
		switchLounge.Close()
		tcpConnHub.Close()
	}()

	// 启动 TCP 服务以接收另一端传输过来的交换数据
	go setUpTCPServer(servPort, tcpConnHub, switchDataChan, errChan, sigCtx)
	// 连接到另一个 switch 节点
	go connectPeer(peerAddr, peerPort, tcpConnHub, switchDataChan, errChan, sigCtx)

	// 把接收到的交换数据写入等候室
	for {
		select {
		case msg := <-multicastChan:
			// 来自组播监听器的交换数据
			if err := switchLounge.Write(msg); err != nil {
				fmt.Printf("Warning: failed to write switch message from multicast to lounge: %v\n", err)
			}
			// 交换数据转换为客户端信息存入本地客户端信息等候室
			// 注意 multicastChan 传递过来的消息一定是本机 LocalSend 客户端发出的
			localSendClientInfo, err := utils.SwitchMessageToLocalSendClientInfo(msg)
			if err != nil {
				fmt.Printf("Warning: failed to convert switch message to local client info: %v\n", err)
				continue
			}
			localClientLounge.Add(localSendClientInfo)
		case msg := <-switchDataChan:
			// 来自 TCP 连接的交换数据
			if err := switchLounge.Write(msg); err != nil {
				fmt.Printf("Warning: failed to write switch message from TCP to lounge: %v\n", err)
			}
		case <-sigCtx.Done():
			// 收到退出信号
			return
		}
	}
}
