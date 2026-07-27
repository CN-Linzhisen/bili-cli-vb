package bilibili

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const wbiIndexURL = "https://api.bilibili.com/x/web-interface/wbi/index"

// mixinKeyEncTab 是固定字符位置表，用于从 img_key+sub_key 中提取字符组成 mixin key
// 来源：B站前端 wbi 模块
var mixinKeyEncTab = [...]int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 37, 12, 44, 56, 7,
	60, 1, 24, 54, 6, 25, 17, 41, 30, 36, 52, 20, 38, 21, 16, 13,
	0, 51, 11, 22, 34, 26, 57, 4, 39, 40, 59, 55, 61,
}

// wbiIndexResponse 定义 Wbi index 接口响应结构
type wbiIndexResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    wbiIndexData `json:"data"`
}

type wbiIndexData struct {
	ImgKey string `json:"img_key"`
	SubKey string `json:"sub_key"`
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

// FetchWbiKeys 从 B站 API 获取最新的 img_key 和 sub_key
func FetchWbiKeys(client *http.Client) (imgKey, subKey string, err error) {
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Get(wbiIndexURL)
	if err != nil {
		return "", "", fmt.Errorf("请求 Wbi index 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取 Wbi index 响应失败: %w", err)
	}

	var result wbiIndexResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("解析 Wbi index 响应失败: %w", err)
	}

	if result.Code != 0 {
		return "", "", fmt.Errorf("Wbi index API 错误 (code=%d): %s", result.Code, result.Message)
	}

	return result.Data.ImgKey, result.Data.SubKey, nil
}

// SignParams 对参数进行 Wbi 签名（自动获取 key 后签名）
func SignParams(client *http.Client, params map[string]string) (map[string]string, error) {
	imgKey, subKey, err := FetchWbiKeys(client)
	if err != nil {
		return nil, err
	}

	return wbiSign(params, imgKey, subKey), nil
}
