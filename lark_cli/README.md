# Lark Client (飞书机器人)

Matterbridge 飞书机器人客户端，用于在飞书和 Matterbridge 之间双向转发消息。

## 功能特性

- ✅ **双向转发**：飞书消息 -> Matterbridge -> 其他平台，反之亦然
- ✅ **自动映射**：使用飞书 Chat ID 作为 Matterbridge Channel
- ✅ **事件驱动**：基于飞书开放平台事件订阅机制（Webhook）
- ✅ **WebSocket 支持**：与 Matterbridge 之间使用 WebSocket 实时通讯

## 前置要求

1. **Matterbridge** 已安装并运行
2. **飞书开放平台应用**：
   - 启用机器人能力
   - 启用事件订阅（消息接收）
   - 获取 `App ID`, `App Secret` 和 `Verification Token`
3. **公网地址**（或内网穿透）：用于飞书事件回调

## 快速开始

### 1. 编译

```bash
cd lark_cli
go build -o lark_cli main.go client.go lark.go
```

### 2. 配置飞书应用

- 在飞书开放平台设置事件订阅地址：`http://your-domain/webhook/event`
- 订阅事件：`im.message.receive_v1` (接收消息 v1)

### 3. 运行

```bash
./lark_cli \
  -api-url="http://127.0.0.1:4242" \
  -token="your-mb-token" \
  -gateway="lark_gateway" \
  -app-id="cli_xxxxxxxx" \
  -app-secret="xxxxxxxxxxxxxxxx" \
  -verify-token="xxxxxxxx" \
  -port=8080
```

## 参数说明

| 参数 | 环境变量 | 说明 | 默认值 |
|------|---------|------|--------|
| `-api-url` | `API_URL` | Matterbridge API 地址 | `http://127.0.0.1:4242` |
| `-token` | `TOKEN` | Matterbridge API Token | - |
| `-gateway` | `GATEWAY` | Matterbridge Gateway 名称 | `lark_gateway` |
| `-app-id` | `APP_ID` | 飞书 App ID | - |
| `-app-secret` | `APP_SECRET` | 飞书 App Secret | - |
| `-verify-token`| `VERIFY_TOKEN` | 飞书事件验证 Token | - |
| `-port` | `PORT` | 本地 Webhook 监听端口 | `8080` |

## 工作原理

1. **飞书 -> Matterbridge**:
   - 用户在飞书发送消息。
   - 飞书发送事件到 `lark_cli` 的 `/webhook/event` 接口。
   - `lark_cli` 将消息转发到 Matterbridge API 的 `POST /api/message`。
   - `lark_cli` 使用 `Chat ID` 作为 `Channel`。

2. **Matterbridge -> 飞书**:
   - Matterbridge 收到其他平台（如 Telegram）的消息。
   - `lark_cli` 通过 WebSocket 收到该消息。
   - `lark_cli` 根据 `Channel` (即 Chat ID) 将消息发送到飞书对应的群聊或私聊。
