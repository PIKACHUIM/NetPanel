package handlers

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"regexp"
)

// MACAddress 6字节MAC地址
type MACAddress [6]byte

// MagicPacket 魔术包
type MagicPacket struct {
	header  [6]byte
	payload [16]MACAddress
}

var reMAC = regexp.MustCompile(`^([0-9a-fA-F]{2}[:\-]){5}([0-9a-fA-F]{2})$`)

// newMagicPacket 创建魔术包
func newMagicPacket(mac string) (*MagicPacket, error) {
	var packet MagicPacket
	var macAddr MACAddress

	hwAddr, err := net.ParseMAC(mac)
	if err != nil {
		return nil, err
	}
	if !reMAC.MatchString(mac) {
		return nil, fmt.Errorf("%s 不是有效的 IEEE 802 MAC-48 地址", mac)
	}
	for idx := range macAddr {
		macAddr[idx] = hwAddr[idx]
	}
	for idx := range packet.header {
		packet.header[idx] = 0xFF
	}
	for idx := range packet.payload {
		packet.payload[idx] = macAddr
	}
	return &packet, nil
}

// marshal 序列化魔术包
func (mp *MagicPacket) marshal() ([]byte, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, mp); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sendWakePacket 发送 WOL 魔术包
func sendWakePacket(macAddr, broadcastIP, iface string, port int) error {
	var localAddr *net.UDPAddr
	if iface != "" {
		ief, err := net.InterfaceByName(iface)
		if err != nil {
			return err
		}
		addrs, err := ief.Addrs()
		if err != nil || len(addrs) == 0 {
			return fmt.Errorf("网络接口 %s 没有可用地址", iface)
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
				localAddr = &net.UDPAddr{IP: ipNet.IP}
				break
			}
		}
	}

	if broadcastIP == "" {
		broadcastIP = "255.255.255.255"
	}
	if port == 0 {
		port = 9
	}

	bcastAddr := fmt.Sprintf("%s:%d", broadcastIP, port)
	udpAddr, err := net.ResolveUDPAddr("udp", bcastAddr)
	if err != nil {
		return err
	}

	mp, err := newMagicPacket(macAddr)
	if err != nil {
		return err
	}
	bs, err := mp.marshal()
	if err != nil {
		return err
	}

	conn, err := net.DialUDP("udp", localAddr, udpAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	n, err := conn.Write(bs)
	if err == nil && n != 102 {
		err = fmt.Errorf("魔术包发送了 %d 字节（期望 102 字节）", n)
	}
	return err
}
