package utils

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword 对密码进行 bcrypt 哈希
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 验证密码是否匹配哈希值（同时兼容明文密码）
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err == nil {
		return true
	}
	// 兼容旧版明文密码
	return password == hash
}

// GenerateKey 生成随机 key
func GenerateKey(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

// ValidatePort 验证端口号
func ValidatePort(port int) bool {
	return port >= 1 && port <= 65535
}

// ValidateIP 验证 IP 地址
func ValidateIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// ValidateCIDR 验证 CIDR
func ValidateCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

// ValidateMAC 验证 MAC 地址
func ValidateMAC(mac string) bool {
	re := regexp.MustCompile(`^([0-9a-fA-F]{2}[:-]){5}([0-9a-fA-F]{2})$`)
	return re.MatchString(mac)
}

// ParsePorts 解析端口字符串（支持单端口、范围、逗号分隔）
func ParsePorts(portsStr string) ([]int, error) {
	var ports []int
	if portsStr == "" {
		return ports, nil
	}

	parts := strings.Split(portsStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				continue
			}
			start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err1 != nil || err2 != nil || start > end {
				continue
			}
			for i := start; i <= end; i++ {
				ports = append(ports, i)
			}
		} else {
			p, err := strconv.Atoi(part)
			if err == nil {
				ports = append(ports, p)
			}
		}
	}
	return ports, nil
}

// GetLocalIPs 获取本机所有 IP 地址
func GetLocalIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ips = append(ips, ip.String())
		}
	}
	return ips
}

// GetNetInterfaces 获取网络接口列表
func GetNetInterfaces() []map[string]string {
	var result []map[string]string
	ifaces, err := net.Interfaces()
	if err != nil {
		return result
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		var ipList []string
		for _, addr := range addrs {
			ipList = append(ipList, addr.String())
		}
		result = append(result, map[string]string{
			"name": iface.Name,
			"ips":  strings.Join(ipList, ","),
		})
	}
	return result
}
