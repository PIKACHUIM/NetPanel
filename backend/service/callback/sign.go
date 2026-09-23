package callback

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// sha256Hex 返回 SHA256 的十六进制摘要。
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// hmacSHA256 计算 HMAC-SHA256。
func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// tc3Authorization 生成腾讯云 API 3.0（TC3-HMAC-SHA256）的 Authorization 头。
//
// 算法参见 https://cloud.tencent.com/document/api/213/30654 ：
//
//	CanonicalRequest = POST\n/\n\n<canonicalHeaders>\n<signedHeaders>\n<sha256(body)>
//	StringToSign     = TC3-HMAC-SHA256\n<timestamp>\n<scope>\n<sha256(canonicalRequest)>
//	Signature        = HMAC 派生链(secretKey -> date -> service -> tc3_request)
//
// 与 handlers 中 DNSPod 的签名实现为同一算法，仅 service 名不同。
// 调用方必须保证请求体字符串与本函数收到的 payload 完全一致，
// 且请求头中的 Content-Type 与 Host 与 canonicalHeaders 一致。
func tc3Authorization(secretID, secretKey, service, host, payload string, ts time.Time) string {
	date := ts.UTC().Format("2006-01-02")
	timestamp := strconv.FormatInt(ts.Unix(), 10)

	canonicalHeaders := "content-type:application/json; charset=utf-8\nhost:" + host + "\n"
	signedHeaders := "content-type;host"
	canonicalRequest := strings.Join([]string{
		"POST", "/", "", canonicalHeaders, signedHeaders, sha256Hex([]byte(payload)),
	}, "\n")

	credentialScope := date + "/" + service + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256", timestamp, credentialScope, sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	secretDate := hmacSHA256([]byte("TC3"+secretKey), date)
	secretService := hmacSHA256(secretDate, service)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))

	return fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		secretID, credentialScope, signedHeaders, signature)
}

// acs3Headers 生成阿里云 OpenAPI V3（ACS3-HMAC-SHA256）所需的请求头（含 Authorization）。
//
// 算法参见阿里云《V3 版本请求结构及签名》：
//
//	CanonicalRequest = POST\n/\n\n<canonicalHeaders>\n<signedHeaders>\n<sha256(body)>
//	StringToSign     = ACS3-HMAC-SHA256\n<sha256(canonicalRequest)>
//	Signature        = HMAC-SHA256(accessKeySecret, StringToSign)
//
// 说明：CanonicalHeaders 每条以 "\n" 结尾，拼接时其后还有分隔用的 "\n"，
// 因此 CanonicalRequest 中会出现一个空行，这是该规范本身的形态。
func acs3Headers(accessKeyID, accessKeySecret, host, action, version string, payload []byte) map[string]string {
	nonce := randomNonce()
	date := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	hashedPayload := sha256Hex(payload)

	canonical := map[string]string{
		"host":                  host,
		"x-acs-action":          action,
		"x-acs-version":         version,
		"x-acs-date":            date,
		"x-acs-signature-nonce": nonce,
		"x-acs-content-sha256":  hashedPayload,
	}

	keys := make([]string, 0, len(canonical))
	for k := range canonical {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var cb strings.Builder
	for _, k := range keys {
		cb.WriteString(k)
		cb.WriteString(":")
		cb.WriteString(strings.TrimSpace(canonical[k]))
		cb.WriteString("\n")
	}
	signedHeaders := strings.Join(keys, ";")

	canonicalRequest := strings.Join([]string{
		"POST", "/", "", cb.String(), signedHeaders, hashedPayload,
	}, "\n")

	stringToSign := "ACS3-HMAC-SHA256\n" + sha256Hex([]byte(canonicalRequest))
	signature := hex.EncodeToString(hmacSHA256([]byte(accessKeySecret), stringToSign))

	return map[string]string{
		"x-acs-action":          action,
		"x-acs-version":         version,
		"x-acs-date":            date,
		"x-acs-signature-nonce": nonce,
		"x-acs-content-sha256":  hashedPayload,
		"Authorization": fmt.Sprintf("ACS3-HMAC-SHA256 Credential=%s,SignedHeaders=%s,Signature=%s",
			accessKeyID, signedHeaders, signature),
	}
}

// randomNonce 生成签名用随机串。
func randomNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(b)
}
