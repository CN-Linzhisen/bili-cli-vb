// Package login 实现 B站扫码登录
package login

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/CN-Linzhisen/bili-cli-vb/internal/session"
	qrcode "github.com/skip2/go-qrcode"
)

const (
	qrGenerateURL = "https://passport.bilibili.com/x/passport-login/web/qrcode/generate"
	qrPollURL     = "https://passport.bilibili.com/x/passport-login/web/qrcode/poll?qrcode_key=%s"

	pollInterval = 3 * time.Second  // 轮询间隔
	pollTimeout  = 120 * time.Second // 总超时

	statusNotScanned    = 86101 // 未扫码
	statusScanned       = 86090 // 已扫码待确认
	statusExpired       = 86038 // 二维码已过期
	statusSuccess       = 0     // 登录成功
)

type qrGenerateResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    qrGenerateData   `json:"data"`
}

type qrGenerateData struct {
	URL       string `json:"url"`
	QRCodeKey string `json:"qrcode_key"`
}

type qrPollResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    qrPollData   `json:"data"`
}

type qrPollData struct {
	URL          string `json:"url"`
	RefreshToken string `json:"refresh_token"`
	Timestamp    int64  `json:"timestamp"`
	Code         int    `json:"code"`
	Message      string `json:"message"`
}

// Login 执行完整的扫码登录流程，返回 session 或错误
func Login() (*session.Session, error) {
	// 1. 获取二维码
	key, qrURL, err := generateQRCode()
	if err != nil {
		return nil, fmt.Errorf("获取二维码失败: %w", err)
	}

	// 2. 显示二维码
	if err := displayQRCode(qrURL); err != nil {
		return nil, fmt.Errorf("显示二维码失败: %w", err)
	}
	fmt.Println("请使用 B站 手机客户端扫描上方二维码登录")

	// 3. 轮询扫码状态
	s, err := pollQRCode(key)
	if err != nil {
		return nil, fmt.Errorf("扫码登录失败: %w", err)
	}

	return s, nil
}

// generateQRCode 请求生成二维码，返回 qrcode_key 和二维码内容 URL
func generateQRCode() (key, url string, err error) {
	resp, err := http.Get(qrGenerateURL)
	if err != nil {
		return "", "", fmt.Errorf("请求二维码生成接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取响应失败: %w", err)
	}

	var result qrGenerateResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != 0 {
		return "", "", fmt.Errorf("API 错误: %s (code=%d)", result.Message, result.Code)
	}

	return result.Data.QRCodeKey, result.Data.URL, nil
}

// displayQRCode 将二维码内容以 ASCII 形式打印到终端
func displayQRCode(url string) error {
	qr, err := qrcode.New(url, qrcode.Low)
	if err != nil {
		return err
	}

	// 使用 ANSI 半块字符渲染二维码（更紧凑）
	art := qr.ToString(false)
	// 移除顶部和底部的空行
	lines := strings.Split(art, "\n")
	for _, line := range lines {
		fmt.Println(line)
	}
	return nil
}

// pollQRCode 轮询二维码状态直到登录成功或超时
func pollQRCode(key string) (*session.Session, error) {
	client := &http.Client{
		// 不跟随重定向，以保留 Set-Cookie 头
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	deadline := time.Now().Add(pollTimeout)

	for time.Now().Before(deadline) {
		code, data, headers, err := pollOnce(client, key)
		if err != nil {
			fmt.Printf("轮询出错: %v，继续等待...\n", err)
			time.Sleep(pollInterval)
			continue
		}

		switch code {
		case statusNotScanned:
			fmt.Print(".")
		case statusScanned:
			fmt.Println("\n✓ 已扫码，请在手机上确认登录")
		case statusExpired:
			return nil, fmt.Errorf("二维码已过期，请重新运行程序")
		case statusSuccess:
			fmt.Println("\n✓ 登录成功！")
			return extractSession(headers, data.RefreshToken)
		default:
			fmt.Printf("\n未知状态码: %d，继续等待...\n", code)
		}

		time.Sleep(pollInterval)
	}

	return nil, fmt.Errorf("扫码超时（%v 内未完成）", pollTimeout)
}

// pollOnce 执行一次轮询，返回状态码、数据、响应头
func pollOnce(client *http.Client, key string) (int, *qrPollData, http.Header, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf(qrPollURL, key), nil)
	if err != nil {
		return 0, nil, nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, err
	}

	var result qrPollResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, nil, nil, err
	}

	if result.Code != 0 {
		return 0, nil, nil, fmt.Errorf("API 错误: %s (code=%d)", result.Message, result.Code)
	}

	return result.Data.Code, &result.Data, resp.Header, nil
}

// extractSession 从轮询成功的响应头中提取 Cookie 凭证
func extractSession(headers http.Header, refreshToken string) (*session.Session, error) {
	s := &session.Session{
		RefreshToken: refreshToken,
		CreatedAt:   time.Now().Unix(),
	}

	// 从 Set-Cookie 头中提取凭证
	for _, cookie := range headers["Set-Cookie"] {
		// 格式: "SESSDATA=xxx; Path=/; ..."
		parts := strings.SplitN(cookie, ";", 2)
		if len(parts) == 0 {
			continue
		}
		kv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
		if len(kv) != 2 {
			continue
		}
		name := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		switch name {
		case "SESSDATA":
			s.Sessdata = value
		case "bili_jct":
			s.BiliJCT = value
		case "DedeUserID":
			s.DedeUserID = value
		case "buvid3":
			if s.Buvid3 == "" {
				s.Buvid3 = value
			}
		}
	}

	// 获取 buvid3（可能在另一个响应头中，如果上面没取到则生成一个随机值）
	if s.Buvid3 == "" {
		for _, cookie := range headers["Set-Cookie"] {
			if strings.HasPrefix(cookie, "buvid3=") {
				parts := strings.SplitN(cookie, ";", 2)
				kv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
				if len(kv) == 2 {
					s.Buvid3 = kv[1]
				}
				break
			}
		}
	}
	if s.Buvid3 == "" {
		s.Buvid3 = session.GenerateBuvid3()
	}

	if s.Sessdata == "" {
		// 尝试从响应头中寻找其他携带 Cookie 的方式
		// 某些版本的 B站 API 在成功时也会通过 JSON 返回 Cookie 信息
		fmt.Println("警告: 未能从响应中提取 SESSDATA，登录可能不完整")
	}

	return s, nil
}
