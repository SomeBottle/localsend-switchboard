package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	"log/slog"

	"github.com/somebottle/localsend-switch/constants"
	"github.com/somebottle/localsend-switch/entities"
	"github.com/somebottle/localsend-switch/services"
	"github.com/somebottle/localsend-switch/utils"
)

func main() {
	// 中断信号处理
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// ------------ 先读取配置
	localSendMulticastAddr := os.Getenv("LOCALSEND_MULTICAST_ADDR") // LocalSend 组播地址
	localSendPort := os.Getenv("LOCALSEND_SERVER_PORT") // LocalSend 组播 / HTTP 端口
	peerAddr := os.Getenv("LOCALSEND_SWITCH_PEER_ADDR")
	peerPort := os.Getenv("LOCALSEND_SWITCH_PEER_PORT")
	servPort := os.Getenv("LOCALSEND_SWITCH_SERV_PORT")
	logDebugFlag := os.Getenv("LOCALSEND_SWITCH_LOG_DEBUG") // 是否启用调试日志, 1 为启用
	logDebug := false
	if logDebugFlag == "1" {
		logDebug = true
	}

	// 尝试从命令行读取配置
	flag.StringVar(&peerAddr, "peer-addr", peerAddr, "Peer address")                                      // 另一个 switch 节点的地址
	flag.StringVar(&peerPort, "peer-port", peerPort, "Peer port (same as service port if not specified)") // 另一个 switch 节点的端口
	flag.StringVar(&servPort, "serv-port", servPort, "Service port (same as peer port if not specified)") // 本地 TCP 服务监听端口
	flag.StringVar(&localSendMulticastAddr, "ls-addr", localSendMulticastAddr, "LocalSend (Multicast) address")
	flag.StringVar(&localSendPort, "ls-port", localSendPort, "LocalSend (Multicast / HTTP) port")
	flag.BoolVar(&logDebug,"debug",logDebug,"Enable debug logging")

	flag.Parse()

	// ------------ 初始化全局日志记录器
	logLevel := slog.LevelInfo
	if logDebug {
		logLevel = slog.LevelDebug
	}
	logger:=slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// 没有配置就用默认值
	if localSendMulticastAddr == "" {
		localSendMulticastAddr = constants.LocalSendDefaultMulticastIPv4
		slog.Debug("Multicast address not provided, using default value: " + localSendMulticastAddr)
	}

	if localSendPort == "" {
		localSendPort = constants.LocalSendDefaultPort
		slog.Debug("Multicast port not provided, using default value: " + localSendPort)
	}

	if peerPort == "" {
		peerPort = servPort
		slog.Debug("Peer port not provided, using service port value", "port", peerPort)
	}

	if peerPort == "" && servPort == "" {
		// 没有配置任何端口，只有组播监听服务会启动
		slog.Warn("Both peer port and service port are not provided, only multicast listener will be set up")
	}

	// 检查是否为 IPv6 地址
	isIpv6, err := utils.IsIpv6(localSendMulticastAddr)
	if err != nil {
		slog.Error("Error parsing IP address", "error", err)
		return
	}
	slog.Debug("Is IPv6", "isIpv6", isIpv6)
	// 获得首选出站 IP 地址
	selfIp, err := utils.GetOutboundIP()
	if err != nil {
		slog.Error("Error getting outbound IP address", "error", err)
		return
	}
	// 获得相应的网络接口
	outBoundInterface, err := utils.GetInterfaceByIP(selfIp)
	if err != nil {
		slog.Error("Error getting outbound network interface", "error", err)
		return
	}
	if outBoundInterface == nil {
		slog.Error("No network interface found for IP address", "ip", selfIp.String())
		return
	}

	slog.Info("Outbound IP address", "ip", selfIp.String())
	slog.Info("Using network interface", "interface", outBoundInterface.Name)

	var network string
	if isIpv6 {
		network = "udp6"
	} else {
		network = "udp4"
	}

	// ------------ 为节点生成一个唯一标识符
	nodeId := utils.GenerateRandomSwitchID()
	slog.Info("Switch Node ID", "nodeId", nodeId)
	// ------------ 加入组播组，接收 LocalSend 的发现 UDP 包
	// 相关协议文档: https://github.com/localsend/protocol
	// 本地组播数据转交通道
	multicastChan := make(chan *entities.SwitchMessage, constants.MulticastChanSize)
	// 出现严重异常时的通知通道
	errChan := make(chan error)
	go services.ListenLocalSendMulticast(nodeId, network, localSendMulticastAddr, localSendPort, outBoundInterface, sigCtx, multicastChan, errChan)

	// ------------ 启动交换服务核心模块
	go services.SetUpSwitchCore(nodeId, peerAddr, peerPort, servPort, sigCtx, multicastChan, localSendPort, errChan)

	// 测试接收数据
	for {
		select {
		case err := <-errChan:
			panic(fmt.Sprintf("Exited with error: %v\n", err))
		case <-sigCtx.Done():
			slog.Info("Shutting down gracefully...")
			// 等待一会儿以确保所有 goroutine 都能退出
			time.Sleep(2 * time.Second)
			return
		}
	}

}
