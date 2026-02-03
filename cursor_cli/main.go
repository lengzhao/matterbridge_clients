package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	apiURL      = flag.String("api-url", "http://127.0.0.1:4242", "Matterbridge API URL")
	token       = flag.String("token", "", "API token (optional)")
	gateway     = flag.String("gateway", "mygateway", "Gateway name")
	botUsername = flag.String("bot-username", "AI Agent", "Bot username")
	agentCmd    = flag.String("agent-cmd", "agent", "Cursor CLI command (agent executable)")
	agentArgs   = flag.String("agent-args", "--print", "Cursor CLI arguments (comma-separated)")
	mode        = flag.String("mode", "websocket", "Mode: websocket or polling")
	interval    = flag.Duration("interval", 2*time.Second, "Polling interval (for polling mode)")
	maxWorkers  = flag.Int("max-workers", 10, "Maximum concurrent message processing (0 = unlimited)")
	debug       = flag.Bool("debug", false, "Enable debug logging")
)

func main() {
	flag.Parse()

	// 设置日志
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// 解析 Cursor CLI 参数
	var args []string
	if *agentArgs != "" {
		args = strings.Split(*agentArgs, ",")
		// 去除空格
		for i := range args {
			args[i] = strings.TrimSpace(args[i])
		}
	}

	// 创建客户端
	client := NewClient(Config{
		APIURL:      *apiURL,
		Token:       *token,
		Gateway:     *gateway,
		BotUsername: *botUsername,
		AgentCmd:    *agentCmd,
		AgentArgs:   args,
		Logger:      logger,
		MaxWorkers:  *maxWorkers,
	})

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("收到停止信号，正在关闭...")
		cancel()
	}()

	// 启动客户端
	logger.Info("启动 Cursor CLI",
		"api_url", *apiURL,
		"gateway", *gateway,
		"bot_username", *botUsername,
		"agent_cmd", *agentCmd,
		"mode", *mode,
		"max_workers", *maxWorkers,
	)

	var err error
	switch *mode {
	case "websocket":
		err = client.StartWebSocket(ctx)
	case "polling":
		err = client.StartPolling(ctx, *interval)
	default:
		logger.Error("未知的模式", "mode", *mode)
		os.Exit(1)
	}

	if err != nil && err != context.Canceled {
		logger.Error("客户端运行错误", "error", err)
		os.Exit(1)
	}

	logger.Info("Cursor CLI 已停止")
}
