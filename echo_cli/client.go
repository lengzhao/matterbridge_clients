package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// FileInfo 表示文件附件信息
type FileInfo struct {
	Name    string `json:"Name"`
	Data    string `json:"Data"` // Base64 编码的数据
	Comment string `json:"Comment"`
	Size    int64  `json:"Size"`
}

// Message 表示 Matterbridge API 消息
type Message struct {
	Text      string                 `json:"text"`
	Username  string                 `json:"username"`
	UserID    string                 `json:"userid"`
	Avatar    string                 `json:"avatar"`
	Account   string                 `json:"account"`
	Protocol  string                 `json:"protocol"`
	Channel   string                 `json:"channel"`
	Gateway   string                 `json:"gateway"`
	ID        string                 `json:"id"`
	ParentID  string                 `json:"parent_id"`
	Timestamp time.Time              `json:"timestamp"`
	Event     string                 `json:"event"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

// Client Matterbridge API 客户端
type Client struct {
	apiURL       string
	token        string
	gateway      string
	botUsername  string
	logger       *slog.Logger
	wsConn       *websocket.Conn
	httpClient   *http.Client
	processedIDs map[string]bool
	mu           sync.RWMutex
	onMessage    func(*Message) // 收到 Matterbridge 消息时的回调
}

// Config 客户端配置
type Config struct {
	APIURL      string
	Token       string
	Gateway     string
	BotUsername string
	Logger      *slog.Logger
	OnMessage   func(*Message)
}

// NewClient 创建新的客户端
func NewClient(cfg Config) *Client {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Client{
		apiURL:       strings.TrimSuffix(cfg.APIURL, "/"),
		token:        cfg.Token,
		gateway:      cfg.Gateway,
		botUsername:  cfg.BotUsername,
		logger:       cfg.Logger,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		processedIDs: make(map[string]bool),
		onMessage:    cfg.OnMessage,
	}
}

// SendMessage 发送消息到 Matterbridge
func (c *Client) SendMessage(text string, channel string, username string, files []FileInfo, extraData map[string]interface{}) error {
	payload := map[string]interface{}{
		"text":     text,
		"username": username,
		"gateway":  c.gateway,
		"channel":  channel,
	}
	
	// 处理 userid 字段：
	// 1. 如果 extraData 中有 lark_chat_id，将其编码到 userid 字段
	// 2. 如果 extraData 中有 userid，直接使用
	if larkChatID, ok := extraData["lark_chat_id"].(string); ok && larkChatID != "" {
		payload["userid"] = "lark_chat:" + larkChatID
		delete(extraData, "lark_chat_id") // 从 extraData 中删除，避免重复
	} else if userid, ok := extraData["userid"].(string); ok && userid != "" {
		payload["userid"] = userid
		delete(extraData, "userid") // 从 extraData 中删除，避免重复
	}
	
	// 处理其他可选字段
	if avatar, ok := extraData["avatar"].(string); ok && avatar != "" {
		payload["avatar"] = avatar
		delete(extraData, "avatar")
	}
	if account, ok := extraData["account"].(string); ok && account != "" {
		payload["account"] = account
		delete(extraData, "account")
	}
	if protocol, ok := extraData["protocol"].(string); ok && protocol != "" {
		payload["protocol"] = protocol
		delete(extraData, "protocol")
	}
	if parentID, ok := extraData["parent_id"].(string); ok && parentID != "" {
		payload["parent_id"] = parentID
		delete(extraData, "parent_id")
	}
	if event, ok := extraData["event"].(string); ok && event != "" {
		payload["event"] = event
		delete(extraData, "event")
	}

	// 合并额外数据
	extraPayload := make(map[string]interface{})
	if len(files) > 0 {
		extraPayload["file"] = files
	}
	for k, v := range extraData {
		extraPayload[k] = v
	}
	if len(extraPayload) > 0 {
		payload["extra"] = extraPayload
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequest("POST", c.apiURL+"/api/message", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ConnectWebSocket 连接到 WebSocket
func (c *Client) ConnectWebSocket(ctx context.Context) error {
	wsURL := strings.Replace(c.apiURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/api/websocket"

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	headers := make(http.Header)
	if c.token != "" {
		headers.Set("Authorization", "Bearer "+c.token)
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return fmt.Errorf("dial websocket: %w", err)
	}

	c.wsConn = conn
	c.logger.Info("WebSocket 连接成功", "url", wsURL)
	return nil
}

// StartWebSocket 启动 WebSocket 模式
func (c *Client) StartWebSocket(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			if c.wsConn != nil {
				c.wsConn.Close()
			}
			return ctx.Err()
		default:
			if c.wsConn == nil {
				if err := c.ConnectWebSocket(ctx); err != nil {
					time.Sleep(5 * time.Second)
					continue
				}
			}

			_, data, err := c.wsConn.ReadMessage()
			if err != nil {
				c.logger.Warn("WebSocket 读取错误，准备重连", "error", err)
				c.wsConn.Close()
				c.wsConn = nil
				time.Sleep(2 * time.Second)
				continue
			}

			var msg Message
			if err := json.Unmarshal(data, &msg); err != nil {
				c.logger.Error("解析消息失败", "error", err)
				continue
			}

			// 忽略自己发送的消息
			if msg.Username == c.botUsername {
				continue
			}

			// 异步处理
			if c.onMessage != nil {
				go c.onMessage(&msg)
			}
		}
	}
}
