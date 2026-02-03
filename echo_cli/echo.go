package main

import (
	"fmt"
	"log/slog"
	"strings"
)

// EchoBot Echo 模块，将所有 Matterbridge 消息原样返回
type EchoBot struct {
	mbClient *Client
	logger   *slog.Logger
	prefix   string // 可选的回显前缀
}

// NewEchoBot 创建新的 Echo 机器人
func NewEchoBot(mbClient *Client, logger *slog.Logger, prefix string) *EchoBot {
	return &EchoBot{
		mbClient: mbClient,
		logger:   logger,
		prefix:   prefix,
	}
}

// HandleMessage 处理来自 Matterbridge 的消息并回显
func (e *EchoBot) HandleMessage(msg *Message) {
	// 记录收到的消息
	e.logger.Info("收到 Matterbridge 消息",
		"id", msg.ID,
		"channel", msg.Channel,
		"username", msg.Username,
		"text", msg.Text,
		"protocol", msg.Protocol,
		"account", msg.Account,
		"gateway", msg.Gateway,
		"event", msg.Event)

	// 构造回显消息
	echoText := e.buildEchoText(msg)

	// 提取文件信息（如果有）
	var files []FileInfo
	if msg.Extra != nil {
		if fileList, ok := msg.Extra["file"].([]interface{}); ok {
			for _, f := range fileList {
				// 处理不同类型的 extra 数据结构
				var fileInfo FileInfo
				fileMap, ok := f.(map[string]interface{})
				if ok {
					fileInfo.Name, _ = fileMap["Name"].(string)
					fileInfo.Data, _ = fileMap["Data"].(string)
					if sz, ok := fileMap["Size"].(float64); ok {
						fileInfo.Size = int64(sz)
					}
				} else {
					// 尝试直接断言
					if fi, ok := f.(FileInfo); ok {
						fileInfo = fi
					}
				}
				if fileInfo.Name != "" {
					files = append(files, fileInfo)
				}
			}
		}
	}

	// 回显消息到同一个 channel
	// 处理 userid 字段：
	// 1. 如果是飞书消息（以 "lark_chat:" 开头），提取 chat_id 并传递
	// 2. 否则，直接传递原始的 userid
	extraData := map[string]interface{}{}
	if strings.HasPrefix(msg.UserID, "lark_chat:") {
		// 飞书消息：提取 chat_id
		chatID := strings.TrimPrefix(msg.UserID, "lark_chat:")
		extraData["lark_chat_id"] = chatID
	} else if msg.UserID != "" {
		// 普通消息：直接传递 userid
		extraData["userid"] = msg.UserID
	}
	
	// 传递其他字段
	if msg.Avatar != "" {
		extraData["avatar"] = msg.Avatar
	}
	if msg.Account != "" {
		extraData["account"] = msg.Account
	}
	if msg.Protocol != "" {
		extraData["protocol"] = msg.Protocol
	}
	if msg.ParentID != "" && !strings.Contains(msg.ParentID, "not-found") {
		// 只传递有效的 parent_id，跳过 Matterbridge 生成的错误值
		extraData["parent_id"] = msg.ParentID
	}
	if msg.Event != "" {
		extraData["event"] = msg.Event
	}
	
	if err := e.mbClient.SendMessage(echoText, msg.Channel, "EchoBot", files, extraData); err != nil {
		e.logger.Error("发送回显消息失败", "error", err, "channel", msg.Channel)
	} else {
		e.logger.Info("已发送回显消息", "channel", msg.Channel, "text", echoText)
	}
}

// buildEchoText 构造回显文本
func (e *EchoBot) buildEchoText(msg *Message) string {
	var parts []string

	// 添加前缀（如果有）
	if e.prefix != "" {
		parts = append(parts, e.prefix)
	}

	// 基本消息信息
	parts = append(parts, fmt.Sprintf("📨 Echo Message [ID: %s]", msg.ID))
	parts = append(parts, fmt.Sprintf("👤 User: %s (%s)", msg.Username, msg.UserID))
	parts = append(parts, fmt.Sprintf("📢 Channel: %s", msg.Channel))
	parts = append(parts, fmt.Sprintf("🔌 Protocol: %s", msg.Protocol))
	parts = append(parts, fmt.Sprintf("🚪 Gateway: %s", msg.Gateway))

	if msg.Account != "" {
		parts = append(parts, fmt.Sprintf("👥 Account: %s", msg.Account))
	}

	if msg.ParentID != "" {
		parts = append(parts, fmt.Sprintf("↩️  Reply to: %s", msg.ParentID))
	}

	if msg.Event != "" {
		parts = append(parts, fmt.Sprintf("🎯 Event: %s", msg.Event))
	}

	// 时间戳
	if !msg.Timestamp.IsZero() {
		parts = append(parts, fmt.Sprintf("🕐 Time: %s", msg.Timestamp.Format("2006-01-02 15:04:05")))
	}

	// 分隔线
	parts = append(parts, "━━━━━━━━━━━━━━━")

	// 原始消息文本
	if strings.TrimSpace(msg.Text) != "" {
		parts = append(parts, fmt.Sprintf("💬 Text:\n%s", msg.Text))
	} else {
		parts = append(parts, "💬 Text: (空消息)")
	}

	// 文件信息
	if msg.Extra != nil {
		if fileList, ok := msg.Extra["file"].([]interface{}); ok && len(fileList) > 0 {
			parts = append(parts, fmt.Sprintf("\n📎 Files: %d", len(fileList)))
			for i, f := range fileList {
				if fileMap, ok := f.(map[string]interface{}); ok {
					name, _ := fileMap["Name"].(string)
					size, _ := fileMap["Size"].(float64)
					parts = append(parts, fmt.Sprintf("  %d. %s (%.2f KB)", i+1, name, size/1024))
				}
			}
		}
	}

	return strings.Join(parts, "\n")
}
