package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

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
	apiURL             string
	token              string
	gateway            string
	botUsername        string
	agentCmd           string
	agentArgs          []string
	logger             *slog.Logger
	wsConn             *websocket.Conn
	httpClient         *http.Client
	processedIDs       map[string]bool
	mu                 sync.RWMutex  // 保护 processedIDs 的并发访问
	maxWorkers         int           // 最大并发处理数
	semaphore          chan struct{} // 信号量控制并发
	baseWorkDir        string        // 工作目录基础路径
	enableDirIsolation bool          // 是否启用工作目录隔离
	fallbackWorkDir    string        // 固定临时目录（程序启动时创建）
}

// Config 客户端配置
type Config struct {
	APIURL      string
	Token       string
	Gateway     string
	BotUsername string
	AgentCmd    string
	AgentArgs   []string
	Logger      *slog.Logger
	MaxWorkers  int // 最大并发处理数，0 表示不限制
	// 工作目录隔离配置
	EnableDirIsolation bool   // 是否启用工作目录隔离（每个用户使用独立文件夹）
	BaseWorkDir        string // 工作目录基础路径（如：/tmp/matterbridge_users）
}

// NewClient 创建新的客户端
func NewClient(cfg Config) *Client {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	maxWorkers := cfg.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 10 // 默认最大并发数为 10
	}

	// 设置默认工作目录基础路径
	baseWorkDir := cfg.BaseWorkDir
	if baseWorkDir == "" {
		baseWorkDir = "/tmp/matterbridge_users"
	}

	// 创建固定临时目录（程序启动时创建）
	fallbackWorkDir := "/tmp/matterbridge_fallback"
	if err := os.MkdirAll(fallbackWorkDir, 0755); err != nil {
		cfg.Logger.Warn("创建固定临时目录失败", "dir", fallbackWorkDir, "error", err)
		// 如果创建失败，fallbackWorkDir 为空，后续会使用当前目录
		fallbackWorkDir = ""
	} else {
		cfg.Logger.Debug("固定临时目录已创建", "dir", fallbackWorkDir)
	}

	client := &Client{
		apiURL:             strings.TrimSuffix(cfg.APIURL, "/"),
		token:              cfg.Token,
		gateway:            cfg.Gateway,
		botUsername:        cfg.BotUsername,
		agentCmd:           cfg.AgentCmd,
		agentArgs:          cfg.AgentArgs,
		logger:             cfg.Logger,
		httpClient:         &http.Client{Timeout: 30 * time.Second},
		processedIDs:       make(map[string]bool),
		maxWorkers:         maxWorkers,
		semaphore:          make(chan struct{}, maxWorkers),
		baseWorkDir:        baseWorkDir,
		enableDirIsolation: cfg.EnableDirIsolation,
		fallbackWorkDir:    fallbackWorkDir,
	}

	// 如果启用了工作目录隔离，创建基础目录
	if cfg.EnableDirIsolation {
		if err := os.MkdirAll(baseWorkDir, 0755); err != nil {
			client.logger.Warn("创建工作目录基础路径失败", "path", baseWorkDir, "error", err)
		} else {
			client.logger.Info("工作目录隔离已启用", "base_dir", baseWorkDir)
		}
	}

	return client
}

// SendMessage 发送消息到 Matterbridge
func (c *Client) SendMessage(text string, originalMsg *Message) error {
	// 使用原始消息的 gateway，如果没有则使用默认的
	gateway := c.gateway
	if originalMsg != nil && originalMsg.Gateway != "" {
		gateway = originalMsg.Gateway
	}

	payload := map[string]interface{}{
		"text":     text,
		"username": c.botUsername,
		"gateway":  gateway,
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
		// Echo KeyAuth 中间件期望 Bearer token 格式
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

	c.logger.Debug("消息发送成功", "text", text, "gateway", gateway)
	return nil
}

// GetMessages 轮询获取消息
func (c *Client) GetMessages() ([]Message, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/messages", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.token != "" {
		// Echo KeyAuth 中间件期望 Bearer token 格式
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var messages []Message
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return messages, nil
}

// ConnectWebSocket 连接到 WebSocket
func (c *Client) ConnectWebSocket(ctx context.Context) error {
	wsURL := strings.Replace(c.apiURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/api/websocket"

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// Echo 的 KeyAuth 中间件期望 Bearer token 格式
	headers := make(http.Header)
	if c.token != "" {
		// 使用 Authorization: Bearer token 格式
		headers.Set("Authorization", "Bearer "+c.token)
	}

	conn, resp, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			c.logger.Error("WebSocket 连接失败",
				"error", err,
				"url", wsURL,
				"status", resp.StatusCode,
				"body", string(body))
		} else {
			// 连接被拒绝或其他网络错误
			if strings.Contains(err.Error(), "connection refused") {
				c.logger.Error("无法连接到服务器",
					"error", err,
					"url", wsURL,
					"提示", "请确保 Matterbridge 正在运行，并且端口正确")
			} else {
				c.logger.Error("WebSocket 连接失败", "error", err, "url", wsURL)
			}
		}
		return fmt.Errorf("dial websocket: %w", err)
	}

	c.wsConn = conn
	c.logger.Info("WebSocket 连接成功", "url", wsURL)
	return nil
}

// ReadWebSocketMessage 从 WebSocket 读取消息
func (c *Client) ReadWebSocketMessage() (*Message, error) {
	if c.wsConn == nil {
		return nil, fmt.Errorf("websocket not connected")
	}

	// 使用 defer recover 捕获 panic
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("WebSocket 读取时发生 panic", "panic", r)
			// 清理连接
			c.CloseWebSocket()
			c.wsConn = nil
		}
	}()

	// 尝试读取消息
	_, data, err := c.wsConn.ReadMessage()
	if err != nil {
		// 检查是否是 "repeated read on failed connection" 错误
		if strings.Contains(err.Error(), "repeated read on failed") {
			c.logger.Warn("检测到连接失败，清理连接", "error", err)
			c.CloseWebSocket()
			c.wsConn = nil
			return nil, fmt.Errorf("websocket connection failed: %w", err)
		}

		// 如果是连接关闭错误
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) ||
			websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
			// 连接已关闭，清理连接
			c.CloseWebSocket()
			c.wsConn = nil
			return nil, fmt.Errorf("websocket closed: %w", err)
		}
		return nil, fmt.Errorf("read message: %w", err)
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal message: %w", err)
	}

	return &msg, nil
}

// CloseWebSocket 关闭 WebSocket 连接
func (c *Client) CloseWebSocket() error {
	if c.wsConn != nil {
		return c.wsConn.Close()
	}
	return nil
}

// ExecuteCursor 执行 Cursor CLI 命令
func (c *Client) ExecuteCursor(ctx context.Context, prompt string) (string, error) {
	return c.ExecuteCursorAsUser(ctx, prompt, "")
}

// getUserWorkDir 获取用户的工作目录
func (c *Client) getUserWorkDir(username string) (string, error) {
	if !c.enableDirIsolation {
		return "", nil // 未启用目录隔离，返回空（使用当前目录）
	}

	// 如果用户名不存在，使用固定临时目录
	if username == "" {
		return c.fallbackWorkDir, nil // 如果临时目录也未创建，返回空（使用当前目录）
	}

	// 清理用户名，防止路径遍历攻击
	safeUsername := filepath.Base(username)
	if safeUsername == "." || safeUsername == ".." {
		safeUsername = "default"
	}

	// 构建用户专属目录路径
	userDir := filepath.Join(c.baseWorkDir, safeUsername)

	// 创建目录（如果不存在）
	if err := os.MkdirAll(userDir, 0755); err != nil {
		// 如果创建失败，使用固定临时目录
		c.logger.Warn("创建用户工作目录失败，使用固定临时目录",
			"username", username,
			"intended_dir", userDir,
			"fallback_dir", c.fallbackWorkDir,
			"error", err,
		)
		return c.fallbackWorkDir, nil
	}

	return userDir, nil
}

// ExecuteCursorAsUser 以指定用户身份执行 Cursor CLI 命令
func (c *Client) ExecuteCursorAsUser(ctx context.Context, prompt string, username string) (string, error) {
	args := append(c.agentArgs, prompt)
	var cmd *exec.Cmd

	// 获取用户工作目录（如果启用了目录隔离）
	workDir, err := c.getUserWorkDir(username)
	if err != nil {
		c.logger.Warn("获取用户工作目录失败，使用当前目录", "username", username, "error", err)
		workDir = ""
	}

	// 创建命令
	cmd = exec.CommandContext(ctx, c.agentCmd, args...)

	// 设置工作目录（如果启用了目录隔离）
	if workDir != "" {
		cmd.Dir = workDir
		c.logger.Debug("设置工作目录", "username", username, "work_dir", workDir)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	c.logger.Info("执行 Cursor CLI 命令", "cmd", c.agentCmd, "args", args, "username", username, "work_dir", workDir)

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("execute cursor: %w, stderr: %s", err, errMsg)
	}

	output := stdout.String()
	c.logger.Debug("Cursor CLI 执行完成", "output_length", len(output), "username", username)
	return output, nil
}

// isMessageProcessed 检查消息是否已处理（线程安全）
func (c *Client) isMessageProcessed(msgID string) bool {
	if msgID == "" {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.processedIDs[msgID]
}

// markMessageProcessed 标记消息为已处理（线程安全）
func (c *Client) markMessageProcessed(msgID string) {
	if msgID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processedIDs[msgID] = true
}

// ProcessMessageAsync 异步处理消息（在 goroutine 中执行）
func (c *Client) ProcessMessageAsync(msg *Message) {
	// 忽略自己发送的消息
	if msg.Username == c.botUsername {
		return
	}

	// 忽略已处理的消息（线程安全检查）
	if c.isMessageProcessed(msg.ID) {
		c.logger.Debug("消息已处理，跳过", "id", msg.ID)
		return
	}

	// 忽略非文本消息（如连接事件等）
	if msg.Event != "" && msg.Event != "user_action" {
		c.logger.Debug("忽略事件消息", "event", msg.Event)
		return
	}

	// 如果消息为空，跳过
	if strings.TrimSpace(msg.Text) == "" {
		return
	}

	// 标记为已处理（在检查后立即标记，避免重复处理）
	c.markMessageProcessed(msg.ID)

	// 在 goroutine 中异步处理
	go c.processMessage(msg)
}

// processMessage 实际处理消息的逻辑（在 goroutine 中执行）
func (c *Client) processMessage(msg *Message) {
	// 获取信号量，控制并发数
	c.semaphore <- struct{}{}
	defer func() { <-c.semaphore }()

	c.logger.Info("开始处理消息", "username", msg.Username, "text", msg.Text, "id", msg.ID)

	// 执行 Cursor CLI 命令
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 使用消息中的用户名执行命令（如果启用了用户隔离）
	response, err := c.ExecuteCursorAsUser(ctx, msg.Text, msg.Username)
	if err != nil {
		c.logger.Error("Cursor CLI 执行失败", "error", err, "id", msg.ID)
		errorMsg := fmt.Sprintf("执行失败: %v", err)
		if err := c.SendMessage(errorMsg, msg); err != nil {
			c.logger.Error("发送错误消息失败", "error", err)
		}
		return
	}

	// 清理响应（移除多余的空行）
	response = strings.TrimSpace(response)
	if response == "" {
		response = "（无响应）"
	}

	// 发送响应，传递原始消息以保留 gateway 信息
	if err := c.SendMessage(response, msg); err != nil {
		c.logger.Error("发送响应失败", "error", err, "id", msg.ID)
		return
	}

	c.logger.Info("响应已发送", "response_length", len(response), "id", msg.ID)
}

// ProcessMessage 同步处理消息（保持向后兼容，但内部使用异步处理）
func (c *Client) ProcessMessage(msg *Message) error {
	c.ProcessMessageAsync(msg)
	return nil
}

// StartPolling 启动轮询模式
func (c *Client) StartPolling(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	c.logger.Info("启动轮询模式", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			messages, err := c.GetMessages()
			if err != nil {
				c.logger.Error("获取消息失败", "error", err)
				continue
			}

			for _, msg := range messages {
				// 异步处理消息，不等待完成
				c.ProcessMessageAsync(&msg)
			}
		}
	}
}

// StartWebSocket 启动 WebSocket 模式
func (c *Client) StartWebSocket(ctx context.Context) error {
	c.logger.Info("启动 WebSocket 模式")

	// 连接循环，支持自动重连
	for {
		select {
		case <-ctx.Done():
			c.CloseWebSocket()
			return ctx.Err()
		default:
			// 如果未连接，尝试连接
			if c.wsConn == nil {
				if err := c.ConnectWebSocket(ctx); err != nil {
					// 连接失败，等待后重试（错误已在 ConnectWebSocket 中记录）
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(5 * time.Second):
						c.logger.Info("正在重试 WebSocket 连接...")
						continue
					}
				}
			}

			// 设置读取超时
			if c.wsConn != nil {
				c.wsConn.SetReadDeadline(time.Now().Add(30 * time.Second))
			}

			msg, err := c.ReadWebSocketMessage()
			if err != nil {
				// ReadWebSocketMessage 已经清理了连接，这里只需要处理重连逻辑

				// 检查是否是连接失败或关闭错误
				if strings.Contains(err.Error(), "websocket closed") ||
					strings.Contains(err.Error(), "websocket connection failed") ||
					websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) ||
					websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					c.logger.Warn("WebSocket 连接已关闭，准备重连", "error", err)
					// 确保连接已清理
					if c.wsConn != nil {
						c.CloseWebSocket()
						c.wsConn = nil
					}
					// 短暂等待后重试连接
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(2 * time.Second):
						continue
					}
				}

				// 检查是否是超时错误（这是正常的，继续等待）
				if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "i/o timeout") {
					c.logger.Debug("WebSocket 读取超时，继续等待")
					continue
				}

				// 其他错误，记录并清理连接
				c.logger.Warn("WebSocket 读取错误，准备重连", "error", err)
				if c.wsConn != nil {
					c.CloseWebSocket()
					c.wsConn = nil
				}
				// 短暂等待后重试
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(2 * time.Second):
					continue
				}
			}

			// 成功读取消息，异步处理
			if msg != nil {
				c.ProcessMessageAsync(msg)
			}
		}
	}
}
