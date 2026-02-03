package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

type LarkBot struct {
	client    *lark.Client
	wsClient  *larkws.Client
	mbClient  *Client
	logger    *slog.Logger
	appID     string
	appSecret string
}

func NewLarkBot(appID, appSecret string, mbClient *Client, logger *slog.Logger) *LarkBot {
	return &LarkBot{
		client:    lark.NewClient(appID, appSecret),
		mbClient:  mbClient,
		logger:    logger,
		appID:     appID,
		appSecret: appSecret,
	}
}

// Start 启动飞书 WebSocket 客户端
func (b *LarkBot) Start(ctx context.Context) error {
	// 创建事件处理器
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(b.handleMessage)

	// 创建 WebSocket 客户端
	b.wsClient = larkws.NewClient(b.appID, b.appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

	b.logger.Info("启动飞书 WebSocket 客户端")

	// 启动 WebSocket 连接
	err := b.wsClient.Start(ctx)
	if err != nil {
		b.logger.Error("启动飞书 WebSocket 失败", "error", err)
		return err
	}

	return nil
}

// handleMessage 处理来自飞书的消息事件
func (b *LarkBot) handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	// 忽略机器人自己的消息
	if event.Event.Sender.SenderId.OpenId == nil || *event.Event.Sender.SenderId.OpenId == "" {
		return nil
	}

	msgType := larkcore.StringValue(event.Event.Message.MessageType)
	contentStr := larkcore.StringValue(event.Event.Message.Content)
	var text string
	var files []FileInfo

	switch msgType {
	case "text":
		var content struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(contentStr), &content); err == nil {
			text = content.Text
		}
	case "image":
		var content struct {
			ImageKey string `json:"image_key"`
		}
		if err := json.Unmarshal([]byte(contentStr), &content); err == nil {
			data, err := b.downloadImage(content.ImageKey)
			if err == nil {
				files = append(files, FileInfo{
					Name: "image.png", // 飞书图片通常没文件名，默认一个
					Data: base64.StdEncoding.EncodeToString(data),
					Size: int64(len(data)),
				})
			} else {
				b.logger.Error("下载飞书图片失败", "error", err)
			}
		}
	case "file":
		var content struct {
			FileKey  string `json:"file_key"`
			FileName string `json:"file_name"`
		}
		if err := json.Unmarshal([]byte(contentStr), &content); err == nil {
			data, err := b.downloadFile(content.FileKey)
			if err == nil {
				files = append(files, FileInfo{
					Name: content.FileName,
					Data: base64.StdEncoding.EncodeToString(data),
					Size: int64(len(data)),
				})
			} else {
				b.logger.Error("下载飞书文件失败", "error", err)
			}
		}
	default:
		text = fmt.Sprintf("(未支持的消息类型: %s)", msgType)
	}

	b.logger.Info("收到飞书消息",
		"type", msgType,
		"chat_id", larkcore.StringValue(event.Event.Message.ChatId),
		"sender_id", larkcore.StringValue(event.Event.Sender.SenderId.OpenId),
		"text", text,
		"files", len(files))

	// 转发给 Matterbridge
	chatID := larkcore.StringValue(event.Event.Message.ChatId)
	username := "Lark_" + larkcore.StringValue(event.Event.Sender.SenderId.OpenId)

	// 将 chat_id 存储到 extraData 中，SendMessage 会将它转存到 parent_id 字段
	extraData := map[string]interface{}{
		"lark_chat_id": chatID,
	}

	if err := b.mbClient.SendMessage(text, chatID, username, files, extraData); err != nil {
		b.logger.Error("转发消息到 Matterbridge 失败", "error", err)
	}

	return nil
}

// SendToLark 将 Matterbridge 消息发送到飞书
func (b *LarkBot) SendToLark(msg *Message) error {
	// 从 userid 字段中解析 chat_id
	// 格式: "lark_chat:oc_xxx"
	chatID := ""
	if strings.HasPrefix(msg.UserID, "lark_chat:") {
		chatID = strings.TrimPrefix(msg.UserID, "lark_chat:")
	}

	if chatID == "" {
		b.logger.Warn("无法从消息中获取飞书 chat_id", "userid", msg.UserID, "channel", msg.Channel, "id", msg.ID)
		return nil
	}

	b.logger.Info("准备发送消息到飞书", "chat_id", chatID, "text", msg.Text)

	// 1. 处理文件附件
	if msg.Extra != nil {
		if files, ok := msg.Extra["file"].([]interface{}); ok {
			for _, f := range files {
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
					// 尝试直接断言（取决于 WebSocket 解析逻辑）
					if fi, ok := f.(FileInfo); ok {
						fileInfo = fi
					}
				}

				// 跳过没有实际数据的文件（这些可能只是用来传递 chat_id 的）
				if fileInfo.Data != "" && fileInfo.Name != "" {
					if err := b.sendFileToLark(chatID, fileInfo); err != nil {
						b.logger.Error("发送文件到飞书失败", "error", err, "file", fileInfo.Name)
					}
				}
			}
		}
	}

	// 2. 发送文本内容 (如果非空)
	if strings.TrimSpace(msg.Text) != "" {
		b.logger.Info("发送文本消息到飞书", "chat_id", chatID, "text", msg.Text)
		content := map[string]string{
			"text": msg.Text,
		}
		contentJSON, _ := json.Marshal(content)

		resp, err := b.client.Im.Message.Create(context.Background(), larkim.NewCreateMessageReqBuilder().
			ReceiveIdType(larkim.ReceiveIdTypeChatId).
			Body(larkim.NewCreateMessageReqBodyBuilder().
				ReceiveId(chatID).
				MsgType(larkim.MsgTypeText).
				Content(string(contentJSON)).
				Build()).
			Build())

		if err != nil {
			return fmt.Errorf("call lark api: %w", err)
		}
		if !resp.Success() {
			return fmt.Errorf("lark api error: %d - %s", resp.Code, resp.Msg)
		}
		b.logger.Info("成功发送文本消息到飞书", "chat_id", chatID)
	}

	return nil
}

func (b *LarkBot) sendFileToLark(chatID string, fi FileInfo) error {
	data, err := base64.StdEncoding.DecodeString(fi.Data)
	if err != nil {
		return fmt.Errorf("decode base64: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(fi.Name))
	isImage := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif"

	if isImage {
		// 上传图片
		resp, err := b.client.Im.Image.Create(context.Background(), larkim.NewCreateImageReqBuilder().
			Body(larkim.NewCreateImageReqBodyBuilder().
				ImageType(larkim.ImageTypeMessage).
				Image(bytes.NewReader(data)).
				Build()).
			Build())
		if err != nil {
			return err
		}
		if !resp.Success() {
			return fmt.Errorf("upload image error: %s", resp.Msg)
		}

		// 发送图片消息
		content := map[string]string{"image_key": larkcore.StringValue(resp.Data.ImageKey)}
		contentJSON, _ := json.Marshal(content)
		_, err = b.client.Im.Message.Create(context.Background(), larkim.NewCreateMessageReqBuilder().
			ReceiveIdType(larkim.ReceiveIdTypeChatId).
			Body(larkim.NewCreateMessageReqBodyBuilder().
				ReceiveId(chatID).
				MsgType(larkim.MsgTypeImage).
				Content(string(contentJSON)).
				Build()).
			Build())
		return err
	}

	// 上传文件
	resp, err := b.client.Im.File.Create(context.Background(), larkim.NewCreateFileReqBuilder().
		Body(larkim.NewCreateFileReqBodyBuilder().
			FileType("stream").
			FileName(fi.Name).
			File(bytes.NewReader(data)).
			Build()).
		Build())
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("upload file error: %s", resp.Msg)
	}

	// 发送文件消息
	content := map[string]string{"file_key": larkcore.StringValue(resp.Data.FileKey)}
	contentJSON, _ := json.Marshal(content)
	_, err = b.client.Im.Message.Create(context.Background(), larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(larkim.MsgTypeFile).
			Content(string(contentJSON)).
			Build()).
		Build())
	return err
}

func (b *LarkBot) downloadImage(imageKey string) ([]byte, error) {
	resp, err := b.client.Im.Image.Get(context.Background(), larkim.NewGetImageReqBuilder().
		ImageKey(imageKey).
		Build())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, fmt.Errorf("download image error: %s", resp.Msg)
	}
	return io.ReadAll(resp.File)
}

func (b *LarkBot) downloadFile(fileKey string) ([]byte, error) {
	// 注意：消息中的文件下载需要使用 GetMessageResource 或 GetFile
	// 这里尝试使用 GetFile
	resp, err := b.client.Im.File.Get(context.Background(), larkim.NewGetFileReqBuilder().
		FileKey(fileKey).
		Build())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, fmt.Errorf("download file error: %s", resp.Msg)
	}
	return io.ReadAll(resp.File)
}
