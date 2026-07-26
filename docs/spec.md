# Spec: BiliBili 直播弹幕监控 CLI

## Problem Statement

B站直播间弹幕是实时互动的重要载体，但目前没有一个轻量的、终端原生的工具可以方便地监控弹幕流。现有的方案要么依赖浏览器（打开直播间页面），要么是大而全的录制/互动套件（BililiveRecorder 等），不适合开发者日常调试、关键词追踪等轻量场景。

用户需要一个**在终端中运行的、扫码登录即用的弹幕监控 CLI**，支持实时展示和关键词高亮。

## Solution

一个 Go 编写的跨平台（首发 Windows）命令行工具 `bili-cli`，通过 Bubble Tea TUI 提供实时弹幕滚动展示。首次使用扫码登录 B站账号，凭证持久化到本地文件，后续免登录。连接直播间后弹幕实时滚动显示，命中用户配置的关键词时以特殊颜色高亮。

## User Stories

1. 作为用户，我希望能通过扫码登录 B站账号，以便 CLI 能获取弹幕数据
2. 作为用户，我希望登录成功后凭证能持久化到本地文件，以便下次启动时无需再次扫码
3. 作为用户，我希望能从 TUI 界面输入直播间房间号，以便开始监听
4. 作为用户，我希望能看到直播间弹幕在终端中实时滚动，以便掌握直播间的实时互动
5. 作为用户，我希望弹幕显示发送者昵称、发送时间和消息内容，以便了解弹幕来源
6. 作为用户，我希望能配置关键词列表，以便监控特定内容的弹幕（如"抽奖"、"红包"）
7. 作为用户，我希望命中关键词的弹幕在 TUI 中以高亮颜色显示，以便快速识别
8. 作为用户，我希望能将弹幕保存为 JSONL 日志文件，以便后续离线分析
9. 作为用户，我希望能通过键盘快捷键退出程序，以便随时停止监控
10. 作为用户，我希望配置文件和凭证文件分开存放，以便安全管理和版本控制
11. 作为用户，我希望 TUI 底部状态栏显示当前直播间号、连接状态和弹幕计数，以便了解运行状态
12. 作为用户，我希望登录凭证过期时有清晰的提示，以便知道需要重新扫码

## Implementation Decisions

### 技术栈

| 决策 | 选择 |
|---|---|
| 语言 | Go |
| TUI 框架 | charmbracelet/bubbletea |
| HTTP 客户端 | 标准库 `net/http` |
| WebSocket | github.com/gorilla/websocket |
| 压缩解压 | 标准库 `compress/zlib` + 可选 `andybalholm/brotli` |
| 二维码生成 | github.com/skip2/go-qrcode + 终端 ASCII 渲染 |
| 配置文件格式 | JSON |
| 日志格式 | JSON Lines (.jsonl) |

### 模块划分

```
bili-cli/
├── cmd/                  # CLI 入口
├── internal/
│   ├── login/            # 扫码登录模块
│   │   ├── qrcode.go     # 获取二维码、展示、轮询
│   │   └── session.go    # 凭证序列化/反序列化、加载/保存
│   ├── bilibili/         # B站 API 客户端
│   │   ├── client.go     # HTTP 客户端（Wbi 签名、Cookie 管理）
│   │   ├── danmuinfo.go  # getDanmuInfo 接口
│   │   └── ws.go         # WebSocket 连接与心跳
│   ├── danmaku/          # 弹幕协议解析
│   │   ├── packet.go     # 二进制包解析（Header、操作码）
│   │   └── command.go    # JSON 命令解析（DANMU_MSG 等）
│   ├── filter/           # 关键词过滤
│   │   └── filter.go     # 纯函数：弹幕文本 vs 关键词列表 → 匹配结果
│   ├── tui/              # Bubble Tea TUI
│   │   ├── model.go      # Bubble Tea Model
│   │   ├── view.go       # 渲染（弹幕滚动列表、状态栏）
│   │   └── update.go     # 消息处理（弹幕到达、键盘事件）
│   └── logger/           # 日志
│       └── jsonl.go      # JSONL 日志写入
├── config.json           # 用户配置文件（关键词等）
├── session.json          # 登录凭证文件（自动生成，不提交到 git）
└── danmaku_<room>.jsonl  # 弹幕日志（自动生成，不提交到 git）
```

### 关键数据流

```
启动 → 加载 session.json
   ↓
   ├── 存在 → 尝试连接（若凭证过期则提示重新扫码）
   └── 不存在 → 扫码登录 → 保存 session.json
       ↓
   进入 TUI → 输入房间号
       ↓
   调用 getDanmuInfo API（Wbi 签名）
       ↓
   WebSocket 连接 → 发送认证包
       ↓
   循环：
     接收二进制包 → zlib 解压 → 解析 JSON 命令 → DanmakuEvent
        ↓
        ├── TUI 渲染（关键词高亮判断）
        └── JSONL 日志写入（可选）
```

### Bilibili 协议要点

- **认证**: 需要 `SESSDATA` + `buvid3` Cookie，Wbi 签名访问 `getDanmuInfo` 获取弹幕服务器地址和 token
- **WebSocket 端点**: `wss://broadcastlv.chat.bilibili.com:2245/sub`
- **心跳**: 每 30 秒发送 opcode 2 包，收到 opcode 8 响应（含人气值）
- **包格式**: 大端序二进制头（总大小/头长/协议版本/操作码/序列号）+ 负载
- **压缩**: protocol version 2 为 zlib，3 为 brotli
- **登录认证**: WebSocket 连接后发送 opcode 7 包，JSON 负载包含 uid、roomid、token

### 关键词配置格式

```json
{
  "keywords": ["抽奖", "红包", "感谢", "主播"],
  "danmaku_log": true,
  "log_dir": "./logs"
}
```

### 凭证配置格式

```json
{
  "sessdata": "...",
  "bili_jct": "...",
  "dede_user_id": "...",
  "refresh_token": "...",
  "buvid3": "..."
}
```

### 二维码登录流程

1. GET `https://passport.bilibili.com/x/passport-login/web/qrcode/generate` → 获取 `qrcode_key` + 二维码 URL
2. 用 `qrcode_key` 生成二维码图片 → 渲染为 ASCII 在终端展示
3. 每 3 秒轮询 `https://passport.bilibili.com/x/passport-login/web/qrcode/poll?qrcode_key=xxx`
4. 状态码：`86101` 未扫码 → 继续轮询；`86090` 已扫码待确认 → 继续轮询；`86038` 过期 → 重新生成；`0` 成功 → 收取 Cookie

## Testing Decisions

### 测试哲学

只测试外部行为，不测试内部实现细节。好的测试是给定输入断言输出，而不是验证某函数被调用了多少次。

### 测试接缝

整个系统切出一个测试接缝：

```
二进制包数据 → DanmakuEvent（结构化弹幕事件）
```

这个接缝之上是纯函数逻辑，以下两个模块值得且容易测试：

1. **`internal/danmaku/packet.go`** — 二进制包解析
   - 给定原始包字节，断言正确解析出操作码和负载
   - 给定 zlib 压缩负载，断言正确解压
   - 给定非法字节，断言返回错误

2. **`internal/filter/filter.go`** — 关键词过滤
   - 给定弹幕文本和关键词列表，断言返回正确的匹配结果和颜色标记
   - 全匹配、部分匹配、无匹配、中文/英文/混合关键词

### 测试框架

Go 标准库 `testing` + `github.com/stretchr/testify/assert`（可选）

## Out of Scope

- 同时监听多个直播间（后续版本）
- 发送弹幕
- 礼物/SC 监控（仅弹幕，MVP 不包含礼物事件渲染）
- 进入/离开房间提示
- Web 管理界面
- 非 Windows 平台优化（首发 Windows，但代码结构保持跨平台友好）
- 弹幕数据分析（词云、统计等）

## Further Notes

- 项目初始化后先完成 GitHub 仓库创建和初始提交
- 第三方 Go 依赖使用 Go Modules 管理
- 不要将 `session.json` 和 `*.jsonl` 提交到 git（需添加到 `.gitignore`）
- 扫码登录的 Wbi 签名实现参考 `SocialSisterYi/bilibili-API-collect` 的文档
- 在 `docs/adr/` 中记录关键架构决策
