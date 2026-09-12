// 云厂商 API 签名实现。回调任务（CF 回源 / 阿里 ESA / 腾讯 EO）写回源地址
// 时需要携带有效签名，此前腾讯 EO 使用占位签名、阿里 ESA 未签名，均必然失败。
package callback

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ===== 腾讯云 API 3.0（TC3-HMAC-SHA256）=====

// tc3Sign 生成腾讯云 TC3-HMAC-SHA256 的 Authorization 头。
// 约定：POST JSON 请求，仅签名 content-type 与 host 两个头（与实际发送一致）。
// 签名算法：https://cloud.tencent.com/document/api/213/30654
func tc3Sign(secretID, secretKey, host, service string, timestamp int64, payload []byte) string {
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")

	// 1. 拼接规范请求串
	payloadHash := sha256.Sum256(payload)
	canonicalRequest := fmt.Sprintf("POST\n/\n\ncontent-type:application/json\nhost:%s\n\ncontent-type;host\n%x",
		host, payloadHash)

	// 2. 拼接待签名字符串
	canonicalHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("TC3-HMAC-SHA256\n%d\n%s/%s/tc3_request\n%x",
		timestamp, date, service, canonicalHash)

	// 3. 计算派生密钥与签名
	kDate := hmacSHA256([]byte("TC3"+secretKey), []byte(date))
	kService := hmacSHA256(kDate, []byte(service))
	kSigning := hmacSHA256(kService, []byte("tc3_request"))
	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	return fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s/%s/tc3_request, SignedHeaders=content-type;host, Signature=%s",
		secretID, date, service, signature)
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// ===== 阿里云 V3 签名（ACS3-HMAC-SHA256）=====

// acs3Sign 对请求计算阿里云 V3 签名，设置 x-acs-* 必需头与 Authorization。
// 约定：业务参数以 query 携带（RPC 风格，Action/Version 同时写入 query 与头，
// 与官方 SDK 行为一致），请求体为空。仅签名 host 与 x-acs-* 五个头。
// 签名算法：https://help.aliyun.com/zh/sdk/product-overview/v3-request-structure-and-signature
func acs3Sign(req *http.Request, accessKeyID, accessKeySecret, action, version string) {
	now := time.Now().UTC()
	dateISO := now.Format("2006-01-02T15:04:05Z")
	nonce := fmt.Sprintf("%d", now.UnixNano())

	req.Header.Set("x-acs-action", action)
	req.Header.Set("x-acs-version", version)
	req.Header.Set("x-acs-date", dateISO)
	req.Header.Set("x-acs-signature-nonce", nonce)

	// 规范化 query：按 key 排序、RFC3986 编码（空格为 %20）
	q := req.URL.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		vals := q[k]
		sort.Strings(vals)
		for _, v := range vals {
			pairs = append(pairs, acs3Escape(k)+"="+acs3Escape(v))
		}
	}
	canonicalQuery := strings.Join(pairs, "&")

	signedHeaders := "host;x-acs-action;x-acs-date;x-acs-signature-nonce;x-acs-version"
	canonicalHeaders := fmt.Sprintf("host:%s\nx-acs-action:%s\nx-acs-date:%s\nx-acs-signature-nonce:%s\nx-acs-version:%s\n",
		req.URL.Host, action, dateISO, nonce, version)

	// 请求体为空：payload 哈希取空串哈希
	payloadHash := sha256.Sum256(nil)
	canonicalRequest := fmt.Sprintf("%s\n/\n%s\n%s%s\n%x",
		req.Method, canonicalQuery, canonicalHeaders, signedHeaders, payloadHash)

	canonicalHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := "ACS3-HMAC-SHA256\n" + hex.EncodeToString(canonicalHash[:])
	signature := hex.EncodeToString(hmacSHA256([]byte(accessKeySecret), []byte(stringToSign)))

	req.Header.Set("Authorization", fmt.Sprintf("ACS3-HMAC-SHA256 Credential=%s, SignedHeaders=%s, Signature=%s",
		accessKeyID, signedHeaders, signature))
}

// acs3Escape 按 RFC3986 百分号编码（Go QueryEscape 把空格编码为 +，需替换为 %20）
func acs3Escape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
