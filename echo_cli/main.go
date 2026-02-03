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
	apiURL      = flag.String("api-url", "http://127.0.0.1:4242", "Matterbridge API URL")
	token       = flag.String("token", "mytoken", "Matterbridge API token")
	gateway     = flag.String("gateway", "mygateway", "Matterbridge Gateway name")
	botUsername = flag.String("bot-username", "EchoBot", "Matterbridge Bot username")

	// Echo 配置
	echoPrefix = flag.String("echo-prefix", "[ECHO]", "Prefix for echo messages")

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

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建 Echo 机器人
	var echoBot *EchoBot

	// 创建 Matterbridge 客户端
	mbClient := NewClient(Config{
		APIURL:      *apiURL,
		Token:       *token,
		Gateway:     *gateway,
		BotUsername: *botUsername,
		Logger:      logger,
		OnMessage: func(msg *Message) {
			if echoBot != nil {
				echoBot.HandleMessage(msg)
			}
		},
	})

	// 初始化 Echo 机器人
	echoBot = NewEchoBot(mbClient, logger, *echoPrefix)

	// 启动 Matterbridge WebSocket 客户端
	go func() {
		logger.Info("连接到 Matterbridge (Echo模式)", "url", *apiURL, "gateway", *gateway)
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
