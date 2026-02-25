package wol

import (
	"bytes"
	"fmt"
	"net"
	"regexp"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var reMAC = regexp.MustCompile(`^([0-9a-fA-F]{2}[:\-]){5}([0-9a-fA-F]{0,4}[0-9a-fA-F])$`)

// Manager WOL 管理器
type Manager struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{db: db, log: log}
}

// Wake 发送 WOL 魔术包唤醒设备
func (m *Manager) Wake(id uint) error {
	var device model.WolDevice
	if err := m.db.First(&device, id).Error; err != nil {
		return fmt.Errorf("设备不存在: %w", err)
	}
	return m.SendMagicPacket(device.MACAddress, device.BroadcastIP, device.NetInterface, device.Port)
}

// SendMagicPacket 发送魔术包
func (m *Manager) SendMagicPacket(macAddr, broadcastIP, iface string, port int) error {
	hwAddr, err := net.ParseMAC(macAddr)
	if err != nil {
		return fmt.Errorf("无效的 MAC 地址: %w", err)
	}

	// 构建魔术包：6字节0xFF + 16次重复MAC
	var buf bytes.Buffer
	header := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	buf.Write(header)
	for i := 0; i < 16; i++ {
		buf.Write(hwAddr)
	}

	// 确定本地地址（可选网卡绑定）
	var localAddr *net.UDPAddr
	if iface != "" {
		ief, err := net.InterfaceByName(iface)
		if err == nil {
			addrs, _ := ief.Addrs()
			for _, addr := range addrs {
				if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
					localAddr = &net.UDPAddr{IP: ipNet.IP}
					break
				}
			}
		}
	}

	if broadcastIP == "" {
		broadcastIP = "255.255.255.255"
	}
	if port == 0 {
		port = 9
	}

	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", broadcastIP, port))
	if err != nil {
		return err
	}

	conn, err := net.DialUDP("udp", localAddr, udpAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	packet := buf.Bytes()
	n, err := conn.Write(packet)
	if err != nil {
		return err
	}
	if n != 102 {
		return fmt.Errorf("发送了 %d 字节，期望 102 字节", n)
	}

	m.log.Infof("[WOL] 已向 %s 发送魔术包，目标 MAC: %s", udpAddr, macAddr)
	return nil
}
