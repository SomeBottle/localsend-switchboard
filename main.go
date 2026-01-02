package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/somebottle/localsend-switch/configs"
	"github.com/somebottle/localsend-switch/entities"
	"github.com/somebottle/localsend-switch/services"
	"github.com/somebottle/localsend-switch/utils"
)

const AppVersion = "1.0.0"

func main() {
	// 中断信号处理
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// 获得进程可执行文件目录，切换到该目录，确保如日志的相对路径能正常解析
	executableDir, err := utils.GetExactExecutableDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get executable directory: %v\n", err)
		return
	}
	// ------------ 先读取配置
	localSendMulticastAddr := os.Getenv("LOCALSEND_MULTICAST_ADDR") // LocalSend 组播地址
	localSendPort := os.Getenv("LOCALSEND_SERVER_PORT")             // LocalSend 组播 / HTTP 端口
	peerAddr := os.Getenv("LOCALSEND_SWITCH_PEER_ADDR")
	peerPort := os.Getenv("LOCALSEND_SWITCH_PEER_PORT")
	servPort := os.Getenv("LOCALSEND_SWITCH_SERV_PORT")
	logDebugFlag := os.Getenv("LOCALSEND_SWITCH_LOG_DEBUG") // 是否启用调试日志, 1 为启用
	logDebug := false
	if logDebugFlag == "1" {
		logDebug = true
	}
	clientBroadcastIntervalStr := os.Getenv("LOCALSEND_SWITCH_CLIENT_BROADCAST_INTERVAL")    // 向所有 peer switch 广播本地客户端的间隔
	clientAliveCheckIntervalStr := os.Getenv("LOCALSEND_SWITCH_CLIENT_ALIVE_CHECK_INTERVAL") // 检测本地客户端存活的间隔
	logFilePath := os.Getenv("LOCALSEND_SWITCH_LOG_FILE_PATH")
	logFileMaxSize := os.Getenv("LOCALSEND_SWITCH_LOG_FILE_MAX_SIZE")
	logFileMaxHistorical := os.Getenv("LOCALSEND_SWITCH_LOG_FILE_MAX_HISTORICAL")
	switchPeerConnectMaxRetriesStr := os.Getenv("LOCALSEND_SWITCH_PEER_CONNECT_MAX_RETRIES")
	workingDir := os.Getenv("LOCALSEND_SWITCH_WORK_DIR")

	// 尝试从命令行读取配置
	flag.StringVar(&peerAddr, "peer-addr", peerAddr, "Peer address")                                      // 另一个 switch 节点的地址
	flag.StringVar(&peerPort, "peer-port", peerPort, "Peer port (same as service port if not specified)") // 另一个 switch 节点的端口
	flag.StringVar(&servPort, "serv-port", servPort, "Service port (same as peer port if not specified)") // 本地 TCP 服务监听端口
	flag.StringVar(&localSendMulticastAddr, "ls-addr", localSendMulticastAddr, "LocalSend (Multicast) address")
	flag.StringVar(&localSendPort, "ls-port", localSendPort, "LocalSend (Multicast / HTTP) port")
	flag.BoolVar(&logDebug, "debug", logDebug, "Enable debug logging")
	flag.StringVar(&clientBroadcastIntervalStr, "client-broadcast-interval", clientBroadcastIntervalStr, "The interval in seconds for broadcasting local clients to all peer switches")
	flag.StringVar(&clientAliveCheckIntervalStr, "client-alive-check-interval", clientAliveCheckIntervalStr, "The interval in seconds for checking local client aliveness")
	flag.StringVar(&logFilePath, "log-file", logFilePath, "Log file path")
	flag.StringVar(&logFileMaxSize, "log-file-max-size", logFileMaxSize, "Log file max size in Bytes before rotation")
	flag.StringVar(&logFileMaxHistorical, "log-file-max-historical", logFileMaxHistorical, "Max number of historical log files to keep")
	flag.StringVar(&switchPeerConnectMaxRetriesStr, "peer-connect-max-retries", switchPeerConnectMaxRetriesStr, "Max retries to connect to peer switch before giving up (set to negative number for infinite retries)")
	flag.StringVar(&workingDir, "work-dir", workingDir, "Working directory (default to executable's directory)")
	// 开机自启选项
	var autoStart string
	flag.StringVar(&autoStart, "autostart", "", "Set auto start on system boot, options: 'enable', 'disable'")

	flag.Parse()

	// ------------ 切换工作目录
	if workingDir == "" {
		workingDir = executableDir
	}
	err = os.Chdir(workingDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to change working directory to %s: %v\n", workingDir, err)
		return
	}

	// ------------ 初始化全局日志记录器
	// 日志相关配置
	if logFilePath != "" {
		configs.SetLogFilePath(logFilePath)
	}
	if logFileMaxSize != "" {
		size, err := strconv.ParseInt(logFileMaxSize, 10, 64)
		if err != nil || size <= 0 {
			fmt.Fprintf(os.Stderr, "Invalid log file max size, should be a positive integer: %v\n", err)
			return
		}
		configs.SetLogMaxSizeBytes(size)
	}
	if logFileMaxHistorical != "" {
		count, err := strconv.ParseInt(logFileMaxHistorical, 10, 32)
		if err != nil || count < 0 {
			fmt.Fprintf(os.Stderr, "Invalid log file max historical count, should be a non-negative integer: %v\n", err)
			return
		}
		configs.SetLogMaxHistoricalFiles(int(count))
	}
	// 日志级别
	logLevel := slog.LevelInfo
	if logDebug {
		logLevel = slog.LevelDebug
	}
	// 日志文件写入器
	logFileWriter, err := utils.NewLogWriter(configs.GetLogFilePath(), configs.GetLogMaxSizeBytes(), configs.GetLogMaxHistoricalFiles())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set up log file writer: %v\n", err)
		return
	}
	// 同时写入 STDOUT 和日志文件
	logger := slog.New(slog.NewTextHandler(
		io.MultiWriter(logFileWriter, os.Stdout),
		&slog.HandlerOptions{
			Level: logLevel,
		},
	))
	slog.SetDefault(logger)

	// ----------- 输出版本信息和工作目录
	slog.Info("LocalSend Switchboard starting...", "version", AppVersion)
	slog.Info("Working directory", "dir", workingDir)

	// ------------ 开机自启设置
	switch autoStart {
	case "enable":
		err := utils.SetAutoStart(true)
		if err != nil {
			slog.Error("Failed to enable autostart", "error", err)
			return
		}
		// 启动后直接退出
		slog.Info("Autostart enabled successfully")
		return
	case "disable":
		err := utils.SetAutoStart(false)
		if err != nil {
			slog.Error("Failed to disable autostart", "error", err)
			return
		}
		// 启动后直接退出
		slog.Info("Autostart disabled successfully")
		return
	case "":
		// 没有传入就正常启动后续服务
	default:
		slog.Error("Invalid value for autostart option, should be 'enable', 'disable' or empty", "input", autoStart)
		return
	}

	// ------------ 配置默认值以及配置检查
	if switchPeerConnectMaxRetriesStr != "" {
		switchPeerConnectMaxRetries, err := strconv.ParseInt(switchPeerConnectMaxRetriesStr, 10, 32)
		if err != nil {
			slog.Error("Invalid value for 'peer-connect-max-retries'", "input", switchPeerConnectMaxRetriesStr, "error", err)
		}
		configs.SetSwitchPeerConnectMaxRetries(int(switchPeerConnectMaxRetries))
	}
	slog.Debug("Switch peer connect max retries", "maxRetries", configs.GetSwitchPeerConnectMaxRetries())
	if clientBroadcastIntervalStr != "" {
		clientBroadcastInterval, err := strconv.ParseInt(clientBroadcastIntervalStr, 10, 32)
		if err != nil || clientBroadcastInterval <= 0 {
			slog.Error("Invalid time interval for 'client-broadcast-interval', should be a positive integer", "input", clientBroadcastIntervalStr, "error", err)
		}
		configs.SetLocalClientBroadcastInterval(int(clientBroadcastInterval))
	}
	slog.Debug("Local client broadcast interval (seconds)", "interval", configs.GetLocalClientBroadcastInterval())

	if clientAliveCheckIntervalStr != "" {
		clientAliveCheckInterval, err := strconv.ParseInt(clientAliveCheckIntervalStr, 10, 32)
		if err != nil || clientAliveCheckInterval <= 0 {
			slog.Error("Invalid time interval for 'client-alive-check-interval', should be a positive integer", "input", clientAliveCheckIntervalStr, "error", err)
		}
		configs.SetLocalClientAliveCheckInterval(int(clientAliveCheckInterval))
	}
	slog.Debug("Local client alive check interval (seconds)", "interval", configs.GetLocalClientAliveCheckInterval())

	if localSendMulticastAddr == "" {
		localSendMulticastAddr = configs.LocalSendDefaultMulticastIPv4
		slog.Debug("Multicast address not provided, using default value: " + localSendMulticastAddr)
	}

	if localSendPort == "" {
		localSendPort = configs.LocalSendDefaultPort
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
	multicastChan := make(chan *entities.SwitchMessage, configs.MulticastChanSize)
	// 出现严重异常时的通知通道
	errChan := make(chan error)
	go services.ListenLocalSendMulticast(nodeId, network, localSendMulticastAddr, localSendPort, outBoundInterface, sigCtx, multicastChan, errChan)

	// ------------ 启动交换服务核心模块
	go services.SetUpSwitchCore(nodeId, peerAddr, peerPort, servPort, sigCtx, multicastChan, localSendPort, errChan)

	// 测试接收数据
	for {
		select {
		case err := <-errChan:
			panic(fmt.Sprintf("Exited with error: %v", err))
		case <-sigCtx.Done():
			slog.Info("Shutting down gracefully...")
			logFileWriter.Close()
			// 等待一会儿以确保所有 goroutine 都能退出
			time.Sleep(2 * time.Second)
			return
		}
	}

}
