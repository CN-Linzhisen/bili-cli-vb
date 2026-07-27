package bilibili

import (
	"net/http"
	"time"
)

// DefaultHTTPClient 返回一个带超时设置的默认 HTTP 客户端（用于 API 调用）
func DefaultHTTPClient() http.Client {
	return http.Client{
		Timeout: 15 * time.Second,
	}
}
