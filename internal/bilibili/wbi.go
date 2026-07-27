package bilibili

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

const wbiNavURL = "https://api.bilibili.com/x/web-interface/nav"

// mixinKeyEncTab 是固定字符位置表，用于从 img_key+sub_key 中提取字符组成 mixin key
// 来源：B站前端 wbi 模块（2025年后更新为 64 位表）
var mixinKeyEncTab = [...]int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
}

// navResponse 定义 /x/web-interface/nav 接口响应结构
type navResponse struct {
	Code    int     `json:"code"`
	Message string  `json:"message"`
	Data    navData `json:"data"`
}

type navData struct {
	WbiImg wbiImg `json:"wbi_img"`
}

// wbiImg 包含 wbi 签名的 img_key 和 sub_key（伪装成 PNG 图片 URL）
type wbiImg struct {
	ImgURL string `json:"img_url"`
	SubURL string `json:"sub_url"`
}

// extractKeyFromURL 从类似 "https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png"
// 的 URL 中提取 key（文件名去掉扩展名）
func extractKeyFromURL(rawURL string) string {
	filename := path.Base(rawURL)
	ext := path.Ext(filename)
	return strings.TrimSuffix(filename, ext)
}

// getMixinKey 使用固定字符位置表从 img_key+sub_key 中提取 mixin key
// 算法：取拼接字符串中指定位置的字符，再截取前 32 位
func getMixinKey(imgKey, subKey string) string {
	combined := imgKey + subKey
	var sb strings.Builder
	for _, pos := range mixinKeyEncTab {
		if pos < len(combined) {
			sb.WriteByte(combined[pos])
		}
	}
	mixed := sb.String()
	if len(mixed) > 32 {
		mixed = mixed[:32]
	}
	return mixed
}

// wbiSign 对参数进行 Wbi 签名，返回带 w_rid 和 wts 的新参数
func wbiSign(params map[string]string, imgKey, subKey string) map[string]string {
	if params == nil {
		params = make(map[string]string)
	}

	mk := getMixinKey(imgKey, subKey)

	// 加入时间戳
	wts := strconv.FormatInt(time.Now().Unix(), 10)
	params["wts"] = wts

	// 按 key 排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接为查询字符串
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(url.QueryEscape(k))
		sb.WriteByte('=')
		sb.WriteString(url.QueryEscape(params[k]))
	}

	// 签名: md5(查询字符串 + mixinKey)
	signStr := sb.String() + mk
	hash := md5.Sum([]byte(signStr))
	wRid := hex.EncodeToString(hash[:])

	params["w_rid"] = wRid
	return params
}

// FetchWbiKeys 从 B站 nav 接口获取最新的 img_key 和 sub_key
func FetchWbiKeys(client *http.Client, cookies ...string) (imgKey, subKey string, err error) {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequest("GET", wbiNavURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置完整的浏览器请求头，避免被 B站 反爬拦截
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Origin", "https://www.bilibili.com")
	for _, c := range cookies {
		if c != "" {
			req.Header.Add("Cookie", c)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("请求 nav 接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取 nav 接口响应失败: %w", err)
	}

	var result navResponse
	if err := json.Unmarshal(body, &result); err != nil {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return "", "", fmt.Errorf("解析 nav 接口响应失败 (HTTP %d, Body=%q): %w",
			resp.StatusCode, preview, err)
	}

	if result.Code != 0 {
		return "", "", fmt.Errorf("nav 接口错误 (code=%d): %s", result.Code, result.Message)
	}

	// 从图片 URL 的文件名中提取 key
	imgKey = extractKeyFromURL(result.Data.WbiImg.ImgURL)
	subKey = extractKeyFromURL(result.Data.WbiImg.SubURL)

	if imgKey == "" || subKey == "" {
		return "", "", fmt.Errorf("从 nav 响应中提取 wbi key 失败 (img=%q, sub=%q)",
			result.Data.WbiImg.ImgURL, result.Data.WbiImg.SubURL)
	}

	return imgKey, subKey, nil
}

// SignParams 对参数进行 Wbi 签名（自动获取 key 后签名）
func SignParams(client *http.Client, params map[string]string, cookies ...string) (map[string]string, error) {
	imgKey, subKey, err := FetchWbiKeys(client, cookies...)
	if err != nil {
		return nil, err
	}

	return wbiSign(params, imgKey, subKey), nil
}
