package bilibili

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const danmuInfoURL = "https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo"

// DanmuInfo 存储 getDanmuInfo 接口响应中的弹幕服务器信息
type DanmuInfo struct {
	HostList []DanmuHost `json:"host_list"`
	Token    string      `json:"token"`
}

// DanmuHost 表示一个弹幕服务器节点
type DanmuHost struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	WssPort int    `json:"wss_port"`
	WSPort  int    `json:"ws_port"`
}

type danmuInfoResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    DanmuInfo  `json:"data"`
}

// FetchDanmuInfo 获取直播间弹幕服务器信息（需要带 Wbi 签名和 Cookie）
func FetchDanmuInfo(client *http.Client, roomID int64, sessData, buvid3 string) (*DanmuInfo, error) {
	if client == nil {
		client = http.DefaultClient
	}

	// Wbi 签名参数（带上 Cookie 以避免被拦截）
	params, err := SignParams(client, map[string]string{
		"id":       fmt.Sprintf("%d", roomID),
		"type":     "0",
	}, fmt.Sprintf("SESSDATA=%s; buvid3=%s", sessData, buvid3))
	if err != nil {
		return nil, fmt.Errorf("Wbi 签名失败: %w", err)
	}

	// 构建带签名的 URL
	req, err := http.NewRequest("GET", danmuInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加查询参数
	q := req.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	// 添加必要的 Cookie
	req.Header.Set("Cookie", fmt.Sprintf(
		"SESSDATA=%s; buvid3=%s",
		sessData, buvid3,
	))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://live.bilibili.com/")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 getDanmuInfo 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result danmuInfoResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("getDanmuInfo API 错误 (code=%d): %s", result.Code, result.Message)
	}

	return &result.Data, nil
}
