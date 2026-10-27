package stun

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
)

func TestBuildBindingRequest(t *testing.T) {
	msg := buildBindingRequest(false, false)

	if len(msg) != 20 {
		t.Fatalf("期望 20 字节，实际 %d", len(msg))
	}
	if got := binary.BigEndian.Uint16(msg[0:2]); got != msgTypeBindingRequest {
		t.Errorf("msgType 期望 0x%04x，实际 0x%04x", msgTypeBindingRequest, got)
	}
	if got := binary.BigEndian.Uint32(msg[4:8]); got != stunMagicCookie {
		t.Errorf("magic cookie 期望 0x%08x，实际 0x%08x", stunMagicCookie, got)
	}
	if got := binary.BigEndian.Uint16(msg[2:4]); got != 0 {
		t.Errorf("无属性时消息长度应为 0，实际 %d", got)
	}
}

func TestBuildBindingRequestWithChangeRequest(t *testing.T) {
	msg := buildBindingRequest(true, true)

	if got := binary.BigEndian.Uint16(msg[2:4]); got != 8 {
		t.Errorf("消息长度字段应为属性区长度 8，实际 %d", got)
	}

	attr := msg[20:]
	if got := binary.BigEndian.Uint16(attr[0:2]); got != attrChangeRequest {
		t.Errorf("属性类型期望 CHANGE-REQUEST(0x0003)，实际 0x%04x", got)
	}
	flags := binary.BigEndian.Uint32(attr[4:8])
	if flags != 0x06 {
		t.Errorf("changeIP|changePort 标志期望 0x06，实际 0x%08x", flags)
	}
}

func TestBuildBindingRequestTransactionIDRandom(t *testing.T) {
	a := buildBindingRequest(false, false)
	b := buildBindingRequest(false, false)
	if bytes.Equal(a[8:20], b[8:20]) {
		t.Error("两次请求的 transactionID 不应相同")
	}
}

// buildSTUNResponse 构造一个带 XOR-MAPPED-ADDRESS 属性的响应（测试辅助）
func buildSTUNResponse(ip net.IP, port int, xor bool) []byte {
	ip = ip.To4()
	if ip == nil {
		panic("测试辅助仅支持 IPv4")
	}
	msg := make([]byte, 20)
	binary.BigEndian.PutUint16(msg[0:2], 0x0101) // Binding Response
	copy(msg[8:20], []byte("testtxn12345"))

	var attrType uint16
	if xor {
		attrType = attrXORMappedAddress
	} else {
		attrType = attrMappedAddress
	}

	addr := make([]byte, 8)
	addr[1] = 0x01 // IPv4
	rawPort := uint16(port)
	if xor {
		rawPort ^= uint16(stunMagicCookie >> 16)
	}
	binary.BigEndian.PutUint16(addr[2:4], rawPort)
	copy(addr[4:8], ip)
	if xor {
		xored := make(net.IP, 4)
		cookie := make([]byte, 4)
		binary.BigEndian.PutUint32(cookie, stunMagicCookie)
		for i := 0; i < 4; i++ {
			xored[i] = ip[i] ^ cookie[i]
		}
		copy(addr[4:8], xored)
	}

	attr := make([]byte, 4+len(addr))
	binary.BigEndian.PutUint16(attr[0:2], attrType)
	binary.BigEndian.PutUint16(attr[2:4], uint16(len(addr)))
	copy(attr[4:], addr)
	msg = append(msg, attr...)
	binary.BigEndian.PutUint16(msg[2:4], uint16(len(msg)-20))
	return msg
}

func TestParseSTUNResponse(t *testing.T) {
	data := buildSTUNResponse(net.IPv4(203, 0, 113, 7), 45678, false)

	msg, err := parseSTUNResponse(data)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if msg.msgType != 0x0101 {
		t.Errorf("msgType 期望 0x0101，实际 0x%04x", msg.msgType)
	}
	if msg.transactionID != [12]byte{'t', 'e', 's', 't', 't', 'x', 'n', '1', '2', '3', '4', '5'} {
		t.Errorf("transactionID 解析错误: %q", msg.transactionID)
	}
	if len(msg.attributes) != 1 {
		t.Errorf("期望 1 个属性，实际 %d", len(msg.attributes))
	}
}

func TestParseSTUNResponseTooShort(t *testing.T) {
	if _, err := parseSTUNResponse(make([]byte, 10)); err == nil {
		t.Error("短于 20 字节的响应应报错")
	}
}

func TestExtractAddressPlain(t *testing.T) {
	data := make([]byte, 4)
	data[0], data[1] = 0x00, 0x01
	binary.BigEndian.PutUint16(data[2:4], 8080)
	data = append(data, 203, 0, 113, 7)

	ip, port, err := extractAddress(data, false)
	if err != nil {
		t.Fatalf("extractAddress 失败: %v", err)
	}
	if ip != "203.0.113.7" || port != 8080 {
		t.Errorf("期望 203.0.113.7:8080，实际 %s:%d", ip, port)
	}
}

func TestExtractAddressXOR(t *testing.T) {
	// RFC 5769 示例：映射地址 192.0.2.1:32853，magic cookie 0x2112A442
	// XOR 后端口 = 0x8055^0x2112 = 0xA147；IP = C0000201^2112A442 = E112A643
	data := []byte{0x00, 0x01, 0xA1, 0x47, 0xE1, 0x12, 0xA6, 0x43}

	ip, port, err := extractAddress(data, true)
	if err != nil {
		t.Fatalf("extractAddress 失败: %v", err)
	}
	if ip != "192.0.2.1" || port != 32853 {
		t.Errorf("期望 192.0.2.1:32853，实际 %s:%d", ip, port)
	}
}

func TestExtractAddressInvalidFamily(t *testing.T) {
	data := make([]byte, 8)
	data[1] = 0x02 // IPv6
	if _, _, err := extractAddress(data, false); err == nil {
		t.Error("IPv6 地址应返回不支持错误")
	}
}

func TestExtractAddressTooShort(t *testing.T) {
	if _, _, err := extractAddress(make([]byte, 4), false); err == nil {
		t.Error("短于 8 字节应报错")
	}
}

func TestGetMappedAddressXORPreferred(t *testing.T) {
	resp, err := parseSTUNResponse(buildSTUNResponse(net.IPv4(203, 0, 113, 7), 45678, true))
	if err != nil {
		t.Fatal(err)
	}
	ip, port, err := getMappedAddress(resp)
	if err != nil {
		t.Fatalf("getMappedAddress 失败: %v", err)
	}
	if ip != "203.0.113.7" || port != 45678 {
		t.Errorf("期望 203.0.113.7:45678，实际 %s:%d", ip, port)
	}
}

func TestGetMappedAddressMissing(t *testing.T) {
	msg := &stunMessage{attributes: map[uint16][]byte{}}
	if _, _, err := getMappedAddress(msg); err == nil {
		t.Error("无映射地址属性时应报错")
	}
}

func TestParseHTTPHeader(t *testing.T) {
	resp := "HTTP/1.1 200 OK\r\nSERVER: Linux UPnP/1.0\r\nLocation: http://192.168.1.1:5431/dyndev/uuid:001\r\n\r\n"
	if got := parseHTTPHeader(resp, "Location"); got != "http://192.168.1.1:5431/dyndev/uuid:001" {
		t.Errorf("Location 解析错误: %q", got)
	}
	if got := parseHTTPHeader(resp, "server"); got != "Linux UPnP/1.0" {
		t.Errorf("大小写不敏感匹配失败: %q", got)
	}
	if got := parseHTTPHeader(resp, "Content-Length"); got != "" {
		t.Errorf("不存在的头应返回空: %q", got)
	}
}

func TestSplitLines(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a\r\nb\nc", []string{"a", "b", "c"}},
		{"a\n\n", []string{"a", ""}},
		{"", nil},
	}
	for _, c := range cases {
		got := splitLines(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitLines(%q) = %v，期望 %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitLines(%q)[%d] = %q，期望 %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestEqualFold(t *testing.T) {
	if !equalFold("Content-Length", "content-length") {
		t.Error("大小写不敏感比较失败")
	}
	if equalFold("abc", "abcd") {
		t.Error("长度不同应返回 false")
	}
	if equalFold("abc", "abd") {
		t.Error("内容不同应返回 false")
	}
}
