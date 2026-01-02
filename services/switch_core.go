package services

// 交换服务核心模块

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/somebottle/localsend-switch/configs"
	"github.com/somebottle/localsend-switch/entities"
	"github.com/somebottle/localsend-switch/utils"
)

// setUpPassiveForwarder 启动被动的交换数据转发器，将接收到的交换数据转发给其他节点，并向远端节点注册本机 LocalSend 客户端信息
//
// SwitchLounge: 交换数据等候室
// localClientLounge: 本地客户端信息等候室
// tcpConnHub: TCP 连接管理器
// httpRequestChan: HTTP 请求发送通道
// errChan: 致命错误通道
// sigCtx: 中断信号上下文
func setUpPassiveForwarder(SwitchLounge *SwitchLounge, localClientLounge *LocalClientLounge, tcpConnHub *TCPConnectionHub, httpRequestChan chan<- *entities.HTTPJsonRequest, errChan chan<- error, sigCtx context.Context) {
	// 构建 HTTP 请求对象的方法
	makeHTTPRequest := func(ip net.IP, port uint16, protocol string, jsonBody []byte) *entities.HTTPJsonRequest {
		// 拼接成 host:port 形式，会自动用方括号包裹可能的 IPv6 地址
		hostPortStr := net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port))
		return &entities.HTTPJsonRequest{
			URL:      fmt.Sprintf("%s://%s/api/localsend/v2/register", protocol, hostPortStr),
			Method:   "POST",
			JsonBody: jsonBody,
			RespChan: nil, // 不需要响应
		}
	}
	// 获得本机 IP 以判断包是不是自己发出的
	selfIp, err := utils.GetOutboundIP()
	if err != nil {
		errChan <- fmt.Errorf("Error getting outbound IP address in passiveforwarder: %w", err)
		return
	}
	for {
		select {
		case <-sigCtx.Done():
			// 收到退出信号
			return
		case switchMsg, ok := <-SwitchLounge.Read():
			if !ok {
				// 等候室关闭，退出
				return
			}
			// 对于每个交换信息，转发给所有连接的节点 (除开其来源节点的连接)
			for _, cwc := range tcpConnHub.GetConnectionsExcept(switchMsg.SourceAddr) {
				// 交换信息 TTL 减一
				switchMsg.Payload.DiscoveryTtl--
				// 如果 TTL 已经为 0，则不再转发，丢弃
				if switchMsg.Payload.DiscoveryTtl <= 0 {
					continue
				}
				slog.Debug("Forwarding switch message", "message", switchMsg, "to", cwc.Conn.RemoteAddr().String())
				// 把交换信息发送到对应的发送通道
				cwc.SendChan <- switchMsg
			}
			// 该发现包的真实发起地址
			var remoteIP net.IP = net.ParseIP(switchMsg.Payload.OriginalAddr)
			if remoteIP == nil {
				// 无法解析包的原始 IP 地址，包无效
				slog.Debug("Warning: failed to parse original address from switch message, ignored", "address", switchMsg.Payload.OriginalAddr)
				continue
			}
			// 每个交换信息，只要其**发起方**不是本机，就同时对其**发起地址**发送注册请求
			// 对其发起地址: 发送本机的 LocalSend 客户端信息
			if !remoteIP.Equal(selfIp) {
				slog.Debug("Received non-local client info", "message", switchMsg.Payload)
				// 转换为 LocalSend 客户端信息
				remoteClientInfo, err := utils.SwitchMessageToLocalSendClientInfo(switchMsg)
				if err != nil {
					slog.Debug("Warning: failed to convert switch message to local client info for HTTP request, ignored", "error", err)
					continue
				}

				// 远端和本机的每一个 LocalSend 客户端都要进行信息交换
				for localClientInfo := range localClientLounge.SyncGet() {
					// 序列化为 JSON
					localJsonPayload, err := json.Marshal(localClientInfo)
					if err != nil {
						slog.Debug("Warning: failed to serialize local client info to JSON for HTTP request, ignored", "error", err)
						continue
					}
					// 在远端客户端注册本地客户端信息
					remoteHttpReq := makeHTTPRequest(remoteIP, remoteClientInfo.Port, remoteClientInfo.Protocol, localJsonPayload)
					slog.Info("Register local client on remote node", "url", remoteHttpReq.URL)
					// 发送 HTTP 请求
					select {
					case httpRequestChan <- remoteHttpReq:
					case <-sigCtx.Done():
						// 收到退出信号
						return
					}
				}
			}
		}
	}
}

// setUpProactiveBroadcaster 启动定时主动广播，定期向已知节点广播本机 LocalSend 客户端信息
//
// nodeId: 本节点唯一标识符
// LocalClientLounge: 本地客户端信息等候室
// tcpConnHub: TCP 连接管理器
// sigCtx: 中断信号上下文
func setUpProactiveBroadcaster(nodeId string, localClientLounge *LocalClientLounge, tcpConnHub *TCPConnectionHub, sigCtx context.Context) {
	// 获得本机 IP
	selfIp, err := utils.GetOutboundIP()
	if err != nil {
		slog.Error("Error getting outbound IP address in proactive broadcaster", "error", err)
		return
	}
	// 定时器
	ticker := time.NewTicker(time.Duration(configs.GetLocalClientBroadcastInterval()) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-sigCtx.Done():
			// 收到退出信号
			return
		case <-ticker.C:
			// 定时广播
			// 先获得本地客户端信息列表
			var numLocalClients, numConnections int = 0, tcpConnHub.NumConnections()
			for localClientInfo := range localClientLounge.SyncGet() {
				numLocalClients++
				localSwitchMsg := utils.PackLocalSendClientInfoIntoSwitchMessage(localClientInfo, nodeId, globalDiscoverySeq.Add(1)-1, selfIp)
				// 对每个已连接的节点发送交换消息
				for _, cwc := range tcpConnHub.GetAllConnections() {
					localSwitchMsg.Payload.DiscoveryTtl--
					cwc.SendChan <- localSwitchMsg
				}
			}
			slog.Debug("Proactively broadcasted local client info to connected switch nodes", "numLocalClients", numLocalClients, "numConnections", numConnections)
		}
	}
}

// setUpClientAliveChecker 启动本地客户端存活检查器，定期向本地 LocalSend 客户端发送 HTTP 探测请求，如果存活会自动加入等候室
//
// 如果没有这个协程，只有被动等待 LocalSend 客户端发送 UDP 发现包才能探测到并加入等候室
//
// localSendPort: 本地 LocalSend 客户端监听的端口
// localClientLounge: 本地客户端信息等候室
// httpRequestChan: 发送 HTTP 请求的通道
// sigCtx: 中断信号上下文
func setUpClientAliveChecker(localSendPort string, localClientLounge *LocalClientLounge, httpRequestChan chan<- *entities.HTTPJsonRequest, sigCtx context.Context) {
	// 构造探测请求的方法
	makeProbeRequest := func(port string, protocol string) (*entities.HTTPJsonRequest, <-chan *entities.HTTPResponse) {
		respChan := make(chan *entities.HTTPResponse, 1)
		return &entities.HTTPJsonRequest{
			URL:      fmt.Sprintf("%s://127.0.0.1:%s/api/localsend/v2/info", protocol, port),
			Method:   "GET",
			RespChan: respChan,
		}, respChan
	}
	// 定时器
	ticker := time.NewTicker(time.Duration(configs.GetLocalClientAliveCheckInterval()) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-sigCtx.Done():
			// 收到退出信号
			return
		case <-ticker.C:
			// 定时探测
			slog.Debug("Proactively checking local client alive status")
			// 同时在 http 和 https 协议上探测
			httpReq, httpRespChan := makeProbeRequest(localSendPort, "http")
			httpsReq, httpsRespChan := makeProbeRequest(localSendPort, "https")
			// 发送请求
			select {
			case httpRequestChan <- httpReq:
			case <-sigCtx.Done():
				return
			}
			select {
			case httpRequestChan <- httpsReq:
			case <-sigCtx.Done():
				return
			}
			// 等待响应
			var respHttp, respHttps *entities.HTTPResponse
			select {
			case respHttp = <-httpRespChan:
			case <-sigCtx.Done():
				return
			}
			select {
			case respHttps = <-httpsRespChan:
			case <-sigCtx.Done():
				return
			}
			if respHttp == nil && respHttps == nil {
				// 探测不到本地客户端存活
				slog.Debug("Local client inactive")
				continue
			}
			// 判断协议
			var protocol string = "https"
			if respHttps == nil {
				respHttps = respHttp
				protocol = "http"
			}
			// 解析响应体
			var localClientInfo entities.LocalSendClientInfo
			if err := json.Unmarshal(respHttps.Body, &localClientInfo); err != nil {
				slog.Debug("Warning: failed to parse local client info from probe response, ignored", "error", err)
				continue
			}
			// 值得注意的是 /v2/info 接口会缺失 Port 和 Protocol 字段，需要补全
			uint16Port, err := utils.ParsePort(localSendPort)
			if err != nil {
				slog.Debug("Warning: failed to parse local send port string to uint16, ignored", "error", err)
				continue
			}
			localClientInfo.Port = uint16Port
			localClientInfo.Protocol = protocol
			// 加入等候室
			localClientLounge.Add(&localClientInfo)
			slog.Info("Local client active", "info", localClientInfo)
		}
	}
}

// SetUpSwitchCore 设置并启动交换服务核心模块
//
// nodeId: 本节点唯一标识符
// peerAddr: 远端 switch 节点地址
// peerPort: 远端 switch 节点端口
// servPort: 本地 switch 服务监听端口
// sigCtx: 中断信号上下文
// multicastChan: 来自组播监听器的交换数据通道
// localSendPort: 本地 LocalSend 监听 (组播 / HTTP) 端口
// errChan: 致命错误通道
func SetUpSwitchCore(nodeId string, peerAddr string, peerPort string, servPort string, sigCtx context.Context, multicastChan <-chan *entities.SwitchMessage, localSendPort string, errChan chan<- error) {
	// 通过 TCP 传输的交换数据通道
	switchDataChan := make(chan *entities.SwitchMessage, configs.SwitchDataReceiveChanSize)
	// 维护 TCP 连接的管理器
	var tcpConnHub *TCPConnectionHub = NewTCPConnectionHub()
	// 维护待转发交换信息的等候室
	var switchLounge *SwitchLounge = NewSwitchLounge()
	// 维护本地客户端信息的等候室
	var localClientLounge *LocalClientLounge = NewLocalClientLounge()
	// 用来发送 HTTP 请求的通道
	httpRequestChan := make(chan *entities.HTTPJsonRequest, configs.HTTPClientWorkerCount*2)
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
	// 启动 HTTP 请求发送器 (多个 worker)
	for range configs.HTTPClientWorkerCount {
		go setUpHTTPSender(httpRequestChan, sigCtx)
	}
	// 启动交换数据转发器
	go setUpPassiveForwarder(switchLounge, localClientLounge, tcpConnHub, httpRequestChan, errChan, sigCtx)
	// 启动定时主动广播器
	go setUpProactiveBroadcaster(nodeId, localClientLounge, tcpConnHub, sigCtx)
	// 启动本地客户端存活探测器
	go setUpClientAliveChecker(localSendPort, localClientLounge, httpRequestChan, sigCtx)

	// 把接收到的交换数据写入等候室
	for {
		select {
		case msg := <-multicastChan:
			// 来自组播监听器的交换数据
			if err := switchLounge.Write(msg); err != nil {
				slog.Debug("Warning: failed to write switch message from multicast to lounge, ignored", "message", msg, "error", err)
				continue
			}
			// 交换数据转换为客户端信息存入本地客户端信息等候室
			// 注意 multicastChan 传递过来的消息一定是本机 LocalSend 客户端发出的
			localSendClientInfo, err := utils.SwitchMessageToLocalSendClientInfo(msg)
			if err != nil {
				slog.Debug("Warning: failed to convert switch message to local client info, ignored", "message", msg, "error", err)
				continue
			}
			localClientLounge.Add(localSendClientInfo)
		case msg := <-switchDataChan:
			// 来自 TCP 连接的交换数据
			if err := switchLounge.Write(msg); err != nil {
				slog.Debug("Warning: failed to write switch message from TCP to lounge, ignored", "message", msg, "error", err)
			}
		case <-sigCtx.Done():
			// 收到退出信号
			return
		}
	}
}
