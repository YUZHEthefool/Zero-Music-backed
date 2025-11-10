package main

import (
	"fmt"
	"log"
	"zero-music/config"
	"zero-music/handlers"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Printf("警告: 加载配置文件失败,使用默认配置: %v", err)
		cfg = config.GetDefaultConfig()
	}

	// 创建 Gin 路由器
	router := gin.Default()

	// 健康检查端点
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Zero Music Server is running",
		})
	})

	// 根路径
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"name":    "Zero Music API",
			"version": "1.0.0",
			"endpoints": []string{
				"GET /health - 健康检查",
				"GET /api/songs - 获取所有歌曲列表",
				"GET /api/song/:id - 获取指定歌曲信息",
				"GET /api/stream/:id - 流式传输音频",
			},
		})
	})

	// 创建处理器
	playlistHandler := handlers.NewPlaylistHandler(cfg)
	streamHandler := handlers.NewStreamHandler(cfg)

	// API 路由组
	api := router.Group("/api")
	{
		// 播放列表相关路由
		api.GET("/songs", playlistHandler.GetAllSongs)
		api.GET("/song/:id", playlistHandler.GetSongByID)

		// 音频流相关路由
		api.GET("/stream/:id", streamHandler.StreamAudio)
	}

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("\n🎵 Zero Music Server 启动中...\n")
	fmt.Printf("服务地址: http://localhost:%d\n", cfg.Server.Port)
	fmt.Printf("音乐目录: %s\n\n", cfg.Music.Directory)

	if err := router.Run(addr); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
