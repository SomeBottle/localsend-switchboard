package services

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/somebottle/localsend-switch/constants"
	"github.com/somebottle/localsend-switch/entities"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// ListenLocalSendMulticast 启动 LocalSend 组播消息监听
//
// networkType: "udp4" 或 "udp6"
// multicastAddr: 组播地址
// multicastPort: 组播端口
// outboundInterface: 出站网络接口
// sigCtx: 中断信号上下文，用于优雅关闭监听
// chanMsg: 传递接收到的组播消息的通道
// errChan: 传递异常的通道，一旦传递，进程即将退出
func ListenLocalSendMulticast(networkType string, multicastAddr string, multicastPort string, outboundInterface *net.Interface, sigCtx context.Context, chanMsg chan<- entities.UDPPacketData, errChan chan<- error) {
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
				pc4, err := net.ListenPacket("udp4", ":"+multicastPort)
				if err != nil {
					return true, fmt.Errorf("Error creating UDP4 packet connection: %w", err)
				}
				p4 := ipv4.NewPacketConn(pc4)
				// 加入组播组
				if err := p4.JoinGroup(outboundInterface, &net.UDPAddr{IP: net.ParseIP(multicastAddr)}); err != nil {
					return true, fmt.Errorf("Error joining IPv4 multicast group: %w", err)
				}
				packetConn = entities.PacketConn{
					IPv4Conn: p4,
					IPv6Conn: nil,
				}
			case "udp6":
				pc6, err := net.ListenPacket("udp6", "[::]:"+multicastPort)
				if err != nil {
					return true, fmt.Errorf("Error creating UDP6 packet connection: %w", err)
				}
				p6 := ipv6.NewPacketConn(pc6)
				// 加入组播组
				if err := p6.JoinGroup(outboundInterface, &net.UDPAddr{IP: net.ParseIP(multicastAddr)}); err != nil {
					return true, fmt.Errorf("Error joining IPv6 multicast group: %w", err)
				}
				packetConn = entities.PacketConn{
					IPv4Conn: nil,
					IPv6Conn: p6,
				}
			}
			defer packetConn.Close()
			fmt.Printf("Joined Multicast Group: %s:%s\n", multicastAddr, multicastPort)
			for {
				select {
				case <-sigCtx.Done():
					// 接到退出信号
					return true, nil
				default:
					// 读取数据
					buf := make([]byte, constants.ReadBufferSize)
					n, remoteAddr, err := packetConn.ReadFrom(buf)
					if err != nil {
						return false, err
					}
					clientIP := remoteAddr.(*net.UDPAddr).IP
					clientPort := remoteAddr.(*net.UDPAddr).Port
					// 发送数据到通道
					data := entities.UDPPacketData{
						SourceIP:   clientIP,
						SourcePort: clientPort,
						Data:       buf[:n],
					}
					chanMsg <- data
				}
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

		fmt.Printf("Restarting multicast listener...\nPrevious error: %v\n", err)
		time.Sleep(constants.MulticastListenRetryInterval * time.Second)
	}

}
