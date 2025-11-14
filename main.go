package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"zero-music/config"
	"zero-music/handlers"

	"github.com/gin-gonic/gin"
)

var (
	logFileHandle *os.File
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "config.json", "指定配置文件的路径。")
	logFile := flag.String("log", "app.log", "指定日志文件的路径。")
	flag.Parse()

	// 初始化日志系统。
	closeLogger := setupLogger(*logFile)
	defer closeLogger()

	log.Println("零音乐服务器正在启动...")

	// 加载应用程序配置。
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("警告: 加载配置文件失败，将使用默认配置: %v", err)
		cfg = config.GetDefaultConfig()
	}

	log.Printf("配置加载成功: 服务器地址=%s:%d, 音乐目录=%s", cfg.Server.Host, cfg.Server.Port, cfg.Music.Directory)

	// 创建 Gin 路由器实例。
	router := gin.Default()

	// # 健康检查端点
	//
	// 提供服务状态和音乐目录可访问性的基本信息。
	router.GET("/health", func(c *gin.Context) {
		// 检查音乐目录是否可访问。
		musicDirAccessible := true
		if _, err := os.Stat(cfg.Music.Directory); err != nil {
			musicDirAccessible = false
		}

		status := "ok"
		httpStatus := http.StatusOK
		if !musicDirAccessible {
			status = "degraded"
			httpStatus = http.StatusServiceUnavailable
		}

		c.JSON(httpStatus, gin.H{
			"status":               status,
			"message":              "零音乐服务器正在运行",
			"music_dir_accessible": musicDirAccessible,
			"music_directory":      cfg.Music.Directory,
		})
	})

	// # API 根端点
	//
	// 显示 API 的基本信息和可用的端点列表。
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"name":    "零音乐 API",
			"version": "1.0.0",
			"endpoints": []string{
				"GET /health - 健康检查",
				"GET /api/songs - 获取所有歌曲列表",
				"GET /api/song/:id - 获取指定歌曲信息",
				"GET /api/stream/:id - 流式传输音频",
			},
		})
	})

	// 初始化 API 处理器。
	playlistHandler := handlers.NewPlaylistHandler(cfg)
	streamHandler := handlers.NewStreamHandler(cfg)

	// # API 路由组
	//
	// 定义所有与 API 功能相关的路由。
	api := router.Group("/api")
	{
		// 播放列表路由
		api.GET("/songs", playlistHandler.GetAllSongs)
		api.GET("/song/:id", playlistHandler.GetSongByID)

		// 音频流路由
		api.GET("/stream/:id", streamHandler.StreamAudio)
	}

	// 启动 HTTP 服务器。
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("零音乐服务器启动中...")
	log.Printf("服务地址: http://localhost:%d", cfg.Server.Port)
	log.Printf("音乐目录: %s", cfg.Music.Directory)

	// 创建 HTTP 服务器实例。
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// 在一个单独的 goroutine 中启动服务器，以避免阻塞主线程。
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 等待中断信号，以实现优雅停机。
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务器...")

	// 创建一个具有超时的上下文，以确保关闭操作在限定时间内完成。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 执行优雅停机。
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("服务器强制关闭: %v", err)
	} else {
		log.Println("服务器已优雅关闭")
	}
}

// setupLogger 配置日志系统，将日志同时输出到标准输出和指定的日志文件。
// 它返回一个函数，用于在程序退出时关闭日志文件。
func setupLogger(logFilePath string) func() {
	// 打开或创建日志文件。
	var err error
	logFileHandle, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("警告: 无法打开日志文件 %s: %v，日志将仅输出到标准输出", logFilePath, err)
		return func() {} // 返回一个空函数，因为没有文件需要关闭。
	}

	// 创建一个将日志写入多个输出（标准输出和文件）的 multi-writer。
	multiWriter := io.MultiWriter(os.Stdout, logFileHandle)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// 返回一个闭包，用于在程序结束时安全地关闭日志文件。
	return func() {
		if logFileHandle != nil {
			log.Println("正在关闭日志文件...")
			logFileHandle.Close()
		}
	}
}
