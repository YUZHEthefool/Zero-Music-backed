package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"zero-music/config"
	"zero-music/handlers"
	"zero-music/logger"
	"zero-music/middleware"
	"zero-music/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

// Params 定义命令行参数
type Params struct {
	ConfigPath string
	LogFile    string
}

// parseFlags 解析命令行参数
func parseFlags() *Params {
	configPath := flag.String("config", "config.json", "指定配置文件的路径。")
	logFile := flag.String("log", "app.log", "指定日志文件的路径。")
	flag.Parse()

	return &Params{
		ConfigPath: *configPath,
		LogFile:    *logFile,
	}
}

// ProvideParams 提供命令行参数
func ProvideParams() *Params {
	return parseFlags()
}

// ProvideConfig 提供配置实例
func ProvideConfig(params *Params) (*config.Config, error) {
	cfg, err := config.Load(params.ConfigPath)
	if err != nil {
		logger.Warnf("加载配置文件失败，将使用默认配置: %v", err)
		return config.GetDefaultConfig(), nil
	}
	return cfg, nil
}

// ProvideScanner 提供音乐扫描器实例
func ProvideScanner(cfg *config.Config) services.Scanner {
	return services.NewMusicScanner(
		cfg.Music.Directory,
		cfg.Music.SupportedFormats,
		cfg.Music.CacheTTLMinutes,
	)
}

// ProvidePlaylistHandler 提供播放列表处理器
func ProvidePlaylistHandler(scanner services.Scanner) *handlers.PlaylistHandler {
	return handlers.NewPlaylistHandler(scanner)
}

// ProvideStreamHandler 提供流处理器
func ProvideStreamHandler(scanner services.Scanner, cfg *config.Config) *handlers.StreamHandler {
	return handlers.NewStreamHandler(scanner, cfg)
}

// ProvideRouter 提供 Gin 路由器
func ProvideRouter(
	cfg *config.Config,
	playlistHandler *handlers.PlaylistHandler,
	streamHandler *handlers.StreamHandler,
) *gin.Engine {
	router := gin.Default()

	// 添加请求 ID 中间件
	router.Use(middleware.RequestID())

	// 健康检查端点
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
			"message":              "zero music服务器正在运行",
			"music_dir_accessible": musicDirAccessible,
			"music_directory":      cfg.Music.Directory,
		})
	})

	// API 根端点
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"name":    "zero music API",
			"version": "1.0.0",
			"endpoints": []string{
				"GET /health - 健康检查",
				"GET /api/songs - 获取所有歌曲列表",
				"GET /api/song/:id - 获取指定歌曲信息",
				"GET /api/stream/:id - 流式传输音频",
			},
		})
	})

	// API 路由组
	api := router.Group("/api")
	{
		// 播放列表路由
		api.GET("/songs", playlistHandler.GetAllSongs)
		api.GET("/song/:id", playlistHandler.GetSongByID)

		// 音频流路由
		api.GET("/stream/:id", streamHandler.StreamAudio)
	}

	return router
}

// ProvideHTTPServer 提供 HTTP 服务器
func ProvideHTTPServer(cfg *config.Config, router *gin.Engine) *http.Server {
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	return &http.Server{
		Addr:    addr,
		Handler: router,
	}
}

// initLogger 初始化日志系统
func initLogger(lc fx.Lifecycle, params *Params) error {
	logFileHandle, err := logger.Init(params.LogFile)
	if err != nil {
		logger.Warnf("日志文件初始化警告: %v", err)
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("zero music服务器正在启动...")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if logFileHandle != nil {
				logger.Info("正在关闭日志文件...")
				if err := logFileHandle.Close(); err != nil {
					logger.Errorf("关闭日志文件时出错: %v", err)
				}
			return nil
		},
	})

	return nil
}

// startHTTPServer 启动 HTTP 服务器
func startHTTPServer(lc fx.Lifecycle, srv *http.Server, cfg *config.Config) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Zero Music 服务器启动中...")
			logger.Infof("服务地址: http://localhost:%d", cfg.Server.Port)
			logger.Infof("音乐目录: %s", cfg.Music.Directory)

			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Errorf("服务器启动失败: %v", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("正在关闭服务器...")
			if err := srv.Shutdown(ctx); err != nil {
				logger.Errorf("服务器强制关闭: %v", err)
				return err
			}
			logger.Info("服务器已优雅关闭")
			return nil
		},
	})
}

func main() {
	app := fx.New(
		// 提供依赖
		fx.Provide(
			ProvideParams,
			ProvideConfig,
			ProvideScanner,
			ProvidePlaylistHandler,
			ProvideStreamHandler,
			ProvideRouter,
			ProvideHTTPServer,
		),
		// 调用初始化函数
		fx.Invoke(
			initLogger,
			startHTTPServer,
		),
	)

	app.Run()
}
