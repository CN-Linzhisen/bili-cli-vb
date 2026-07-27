package openlive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultBaseURL = "https://live-open.biliapi.com"

// Client 开放平台 API 客户端
type Client struct {
	httpClient      *http.Client
	baseURL         string
	accessKeyID     string
	accessKeySecret string
	appID           int64
	code            string
}

// NewClient 创建开放平台客户端
func NewClient(httpClient *http.Client, appID int64, accessKeyID, accessKeySecret, code string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		httpClient:      httpClient,
		baseURL:         defaultBaseURL,
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		appID:           appID,
		code:            code,
	}
}

// post 发送带签名的 POST 请求
func (c *Client) post(path string, reqBody, respBody interface{}) error {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	request, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 生成签名头
	headers := BuildSignedHeaders(SignParams{
		AccessKeyID:     c.accessKeyID,
		AccessKeySecret: c.accessKeySecret,
		Body:            bodyBytes,
	})
	request.Header = headers

	// 发送请求
	resp, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	// 先检查是否 HTTP 错误
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respData))
	}

	// 解析基础响应
	var base struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respData, &base); err != nil {
		return fmt.Errorf("解析响应 JSON 失败: %s", string(respData))
	}
	if base.Code != 0 {
		return fmt.Errorf("API 错误 (code=%d): %s", base.Code, base.Message)
	}

	// 解析完整响应
	if respBody != nil {
		if err := json.Unmarshal(respData, respBody); err != nil {
			return fmt.Errorf("解析响应数据失败: %w", err)
		}
	}

	return nil
}

// Start 调用 /v2/app/start 获取 WebSocket 连接信息
func (c *Client) Start() (*StartData, error) {
	req := map[string]interface{}{
		"code":   c.code,
		"app_id": c.appID,
	}
	var resp StartResponse
	if err := c.post("/v2/app/start", req, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// Heartbeat 调用 /v2/app/heartbeat 保持项目心跳
func (c *Client) Heartbeat(gameID string) error {
	req := map[string]string{
		"game_id": gameID,
	}
	return c.post("/v2/app/heartbeat", req, nil)
}

// End 调用 /v2/app/end 关闭项目
func (c *Client) End(gameID string) error {
	req := map[string]interface{}{
		"game_id": gameID,
		"app_id":  c.appID,
	}
	return c.post("/v2/app/end", req, nil)
}
