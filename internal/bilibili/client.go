package bilibili

import (
	"net/http"
	"time"
)

// DefaultHTTPClient 返回一个带超时设置的默认 HTTP 客户端
func DefaultHTTPClient() http.Client {
	return http.Client{
		Timeout: 15 * time.Second,
		// 不跟随重定向（为了手动处理 Set-Cookie）
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
