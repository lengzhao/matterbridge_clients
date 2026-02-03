package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

var (
	// Matterbridge 配置
	apiURL      = flag.String("api-url", "http://127.0.0.1:4243", "Matterbridge API URL")
	token       = flag.String("token", "mytoken", "Matterbridge API token")
	gateway     = flag.String("gateway", "mygateway", "Matterbridge Gateway name")
	botUsername = flag.String("bot-username", "LarkBot", "Matterbridge Bot username")

	// 飞书配置
	appID     = flag.String("app-id", "", "Lark App ID")
	appSecret = flag.String("app-secret", "", "Lark App Secret")

	debug = flag.Bool("debug", false, "Enable debug logging")
)

func main() {
	flag.Parse()

	// 设置日志
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

	if *appID == "" || *appSecret == "" {
		logger.Error("必须提供 Lark App ID 和 App Secret")
		os.Exit(1)
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. 创建 Matterbridge 客户端
	// 我们需要将收到的 Matterbridge 消息发送到飞书，所以需要一个回调
	var bot *LarkBot
	mbClient := NewClient(Config{
		APIURL:      *apiURL,
		Token:       *token,
		Gateway:     *gateway,
		BotUsername: *botUsername,
		Logger:      logger,
		OnMessage: func(msg *Message) {
			if bot != nil {
				if err := bot.SendToLark(msg); err != nil {
					logger.Error("发送消息到飞书失败", "error", err, "id", msg.ID)
				}
			}
		},
	})

	// 2. 创建飞书机器人
	bot = NewLarkBot(*appID, *appSecret, mbClient, logger)

	// 3. 启动飞书 WebSocket 客户端
	go func() {
		logger.Info("启动飞书 WebSocket 客户端")
		if err := bot.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("飞书 WebSocket 客户端运行错误", "error", err)
			cancel()
		}
	}()

	// 4. 启动 Matterbridge WebSocket 客户端
	go func() {
		logger.Info("连接到 Matterbridge", "url", *apiURL, "gateway", *gateway)
		if err := mbClient.StartWebSocket(ctx); err != nil && err != context.Canceled {
			logger.Error("Matterbridge 客户端运行错误", "error", err)
			cancel()
		}
	}()

	// 处理退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("正在关闭...")
	cancel()
	logger.Info("已停止")
}
