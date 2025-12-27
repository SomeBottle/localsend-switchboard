package utils

import (
	"log"
	"net"
)

// IsIpv6 判断给定的地址是否为 IPv6 地址
//
// 返回 (bool, error)：如果是 IPv6 地址返回 true，否则返回 false；如果地址无效，返回错误
func IsIpv6(address string) (bool, error) {
	ip := net.ParseIP(address)
	if ip == nil {
		return false, nil // 无效的 IP 地址
	}
	return ip.To4() == nil, nil
}

// GetOutboundIP 获取本机的首选出站 IP 地址 (而不是 Docker, 虚拟网卡等)
func GetOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal("Failed to get outbound IP address:", err)
		return nil, err
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

// GetInterfaceByIP 根据给定的 IP 地址获取对应的网络接口
// 返回 (*net.Interface, error)：找到的网络接口指针，如果未找到则返回 nil；如果发生错误，返回错误
func GetInterfaceByIP(ip net.IP) (*net.Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iFace := range interfaces {
		addrs, err := iFace.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.Equal(ip) {
				return &iFace, nil
			}
		}
	}
	return nil, nil
}


// WriteAll 确保将所有数据写入到连接中
//
// conn: 目标连接
// data: 要写入的数据切片
func WriteAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}