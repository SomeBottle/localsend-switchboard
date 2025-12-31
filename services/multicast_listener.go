package services

import (
	"context"
	"fmt"
	"net"
	"time"
	"log/slog"

	"github.com/somebottle/localsend-switch/constants"
	"github.com/somebottle/localsend-switch/entities"
	switchdata "github.com/somebottle/localsend-switch/generated/switchdata/v1"
	"github.com/somebottle/localsend-switch/utils"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"google.golang.org/protobuf/encoding/protojson"
)

// ListenLocalSendMulticast 启动 LocalSend 组播消息监听
//
// 注：只接收本地客户端发出的组播包，如果是别的主机发出的组播包会被忽略
//
// nodeId: 本节点的唯一标识符
// networkType: "udp4" 或 "udp6"
// localSendAddr: LocalSend (组播) 地址
// localSendPort: LocalSend (组播 / HTTP) 端口
// outboundInterface: 出站网络接口
// sigCtx: 中断信号上下文，用于优雅关闭监听
// chanMsg: 传递接收到的组播消息的通道
// errChan: 传递异常的通道，一旦传递，进程即将退出
func ListenLocalSendMulticast(nodeId string, networkType string, localSendAddr string, localSendPort string, outboundInterface *net.Interface, sigCtx context.Context, chanMsg chan<- *entities.SwitchMessage, errChan chan<- error) {
	// 获得本机的首选出站 IP 地址，用于过滤掉自己发送的组播消息
	selfIp, err := utils.GetOutboundIP()
	if err != nil {
		errChan <- fmt.Errorf("Error getting outbound IP address: %w", err)
		return
	}
	// protojson 解析器设置
	jsonUnmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: true, // 丢弃未知字段
	}
	// for 循环保持连接
	for {
		exit, err := func() (bool, error) {
			// 这部分要感谢 StackOverflow 这个贴: https://stackoverflow.com/questions/35300039/in-golang-how-to-receive-multicast-packets-with-socket-bound-to-specific-addres
			// 直接用 net.ListenMulticastUDP 没法收到 UDP 包
			// 只能这样先绑定 0.0.0.0:port，然后再加入组播组

			// ------------ 加入组播组 (IPv4 or IPv6)
			var packetConn entities.PacketConn
			switch networkType {
			case "udp4":
				pc4, err := utils.ListenPacketWithREUSEADDR("udp4", ":"+localSendPort)
				if err != nil {
					return true, fmt.Errorf("Error creating UDP4 packet connection: %w", err)
				}
				p4 := ipv4.NewPacketConn(pc4)
				// 加入组播组
				if err := p4.JoinGroup(outboundInterface, &net.UDPAddr{IP: net.ParseIP(localSendAddr)}); err != nil {
					return true, fmt.Errorf("Error joining IPv4 multicast group: %w", err)
				}
				packetConn = entities.PacketConn{
					IPv4Conn: p4,
					IPv6Conn: nil,
				}
			case "udp6":
				pc6, err := utils.ListenPacketWithREUSEADDR("udp6", "[::]:"+localSendPort)
				if err != nil {
					return true, fmt.Errorf("Error creating UDP6 packet connection: %w", err)
				}
				p6 := ipv6.NewPacketConn(pc6)
				// 加入组播组
				if err := p6.JoinGroup(outboundInterface, &net.UDPAddr{IP: net.ParseIP(localSendAddr)}); err != nil {
					return true, fmt.Errorf("Error joining IPv6 multicast group: %w", err)
				}
				packetConn = entities.PacketConn{
					IPv4Conn: nil,
					IPv6Conn: p6,
				}
			}
			// 通知协程停止的通道
			listenerDone := make(chan struct{})
			// ------------ 资源释放
			defer func() {
				close(listenerDone)
				packetConn.Close()
			}()
			// ------------ 创建协程来监听中断信号
			go func() {
				select {
				case <-sigCtx.Done():
					// 接到退出信号，关闭连接，终止服务
					packetConn.Close()
				case <-listenerDone:
					// 退出协程
					return
				}
			}()
			slog.Info("Joined Multicast Group", "address", localSendAddr, "port", localSendPort)
			for {
				// 设置超时时间防止阻塞过久
				if err := packetConn.SetReadDeadline(time.Now().Add(constants.MulticastReadTimeout * time.Second)); err != nil {
					return false, fmt.Errorf("Error setting read deadline: %w", err)
				}
				// 读取数据
				buf := make([]byte, constants.MulticastReadBufferSize)
				// UDP 中一次会读取整个数据报，直接 ReadFrom 即可
				n, remoteAddr, err := packetConn.ReadFrom(buf)
				if err != nil {
					if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
						// 读取超时罢了，继续等待
						continue
					}
					// 如果是被中断，退出
					if sigCtx.Err() != nil {
						slog.Debug("Multicast listener exiting gracefully...\n")
						return true, nil
					}
					// 否则重启服务
					return false, err
				}
				// 解析数据
				discoveryMsg := switchdata.DiscoveryMessage{}
				slog.Debug("Received UDP packet", "from", remoteAddr.String(), "data", string(buf[:n]))
				// 因为 discoveryMsg 是 protobuf 格式，所以用 protojson 解析
				if err := jsonUnmarshaler.Unmarshal(buf[:n], &discoveryMsg); err != nil {
					slog.Debug("Warning: Failed to unmarshal discovery message, ignored", "from", remoteAddr.String(), "error", err)
					continue
				}
				clientIP := remoteAddr.(*net.UDPAddr).IP
				// 过滤掉不是自己的消息，我需要把自己的发现包递交给其他人，如果其他人的发现包能组播到我这里，那不万事大吉了，没必要把他们的发现包再发回去
				if !clientIP.Equal(selfIp) {
					// 不是自己组播的消息，直接忽略
					continue
				}
				discoveryMsg.SwitchId = nodeId
				discoveryMsg.DiscoverySeq = globalDiscoverySeq.Load()
				discoveryMsg.DiscoveryTtl = constants.MaxDiscoveryMessageTTL
				// 在包中塞入原始发送者 IP 地址
				discoveryMsg.OriginalAddr = clientIP.String()
				// 序号递增
				globalDiscoverySeq.Add(1)
				// 包装成 SwitchMessage
				switchMsg := &entities.SwitchMessage{
					SourceAddr: remoteAddr,
					Payload:    &discoveryMsg,
				}
				chanMsg <- switchMsg
			}
		}()
		if exit {
			// 接到退出信号
			if err != nil {
				// 异常退出
				errChan <- err
			}
			break
		}

		slog.Info("Restarting multicast listener", "previousError", err)
		time.Sleep(constants.MulticastListenRetryInterval * time.Second)
	}

}
