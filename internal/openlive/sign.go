package openlive

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SignParams 签名参数
type SignParams struct {
	AccessKeyID     string
	AccessKeySecret string
	Body            []byte
}

// BuildSignedHeaders 生成 B站开放平台 API 签名并返回完整请求头
func BuildSignedHeaders(params SignParams) http.Header {
	headers := http.Header{}

	bodyMD5 := computeMD5(params.Body)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := generateNonce()

	// 构建待签名字典（按 key 排序）
	signMap := map[string]string{
		"x-bili-accesskeyid":        params.AccessKeyID,
		"x-bili-content-md5":        bodyMD5,
		"x-bili-signature-method":   "HMAC-SHA256",
		"x-bili-signature-nonce":    nonce,
		"x-bili-signature-version":  "1.0",
		"x-bili-timestamp":          timestamp,
	}

	keys := make([]string, 0, len(signMap))
	for k := range signMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接待签名字符串: key1:value1\nkey2:value2\n...
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(k)
		sb.WriteByte(':')
		sb.WriteString(signMap[k])
	}
	stringToSign := sb.String()

	// HMAC-SHA256 签名
	mac := hmac.New(sha256.New, []byte(params.AccessKeySecret))
	mac.Write([]byte(stringToSign))
	signature := hex.EncodeToString(mac.Sum(nil))

	// 设置请求头
	headers.Set("Accept", "application/json")
	headers.Set("Content-Type", "application/json")
	headers.Set("x-bili-content-md5", bodyMD5)
	headers.Set("x-bili-timestamp", timestamp)
	headers.Set("x-bili-signature-method", "HMAC-SHA256")
	headers.Set("x-bili-signature-nonce", nonce)
	headers.Set("x-bili-accesskeyid", params.AccessKeyID)
	headers.Set("x-bili-signature-version", "1.0")
	headers.Set("Authorization", signature)

	return headers
}

// computeMD5 计算 MD5 并返回十六进制小写字符串
func computeMD5(data []byte) string {
	if len(data) == 0 {
		// 空字符串的 MD5
		return "d41d8cd98f00b204e9800998ecf8427e"
	}
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// generateNonce 生成随机 nonce（UUID 风格）
func generateNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 极端情况回退
		return fmt.Sprintf("nonce%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // UUID version 4
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
