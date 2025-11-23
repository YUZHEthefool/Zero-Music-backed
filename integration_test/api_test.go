package integration_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"zero-music/config"
	"zero-music/handlers"
	"zero-music/middleware"
	"zero-music/services"

	"github.com/gin-gonic/gin"
)

// setupTestServer 创建并配置测试服务器
func setupTestServer(t *testing.T) (*gin.Engine, string) {
	t.Helper()

	// 创建临时测试目录
	testDir := t.TempDir()

	// 创建测试音乐目录
	musicDir := filepath.Join(testDir, "music")
	if err := os.MkdirAll(musicDir, 0755); err != nil {
		t.Fatalf("创建测试音乐目录失败: %v", err)
	}

	// 创建测试音乐文件
	testMP3Path := filepath.Join(musicDir, "test.mp3")
	if err := os.WriteFile(testMP3Path, []byte("test mp3 content"), 0644); err != nil {
		t.Fatalf("创建测试 MP3 文件失败: %v", err)
	}

	// 配置测试配置
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:         "localhost",
			Port:         8080,
			MaxRangeSize: 100 * 1024 * 1024,
		},
		Music: config.MusicConfig{
			Directory:        musicDir,
			SupportedFormats: []string{".mp3", ".flac", ".wav"},
			CacheTTLMinutes:  5,
		},
	}

	// 设置 Gin 为测试模式
	gin.SetMode(gin.TestMode)

	// 创建路由器
	router := gin.New()
	router.Use(middleware.RequestID())

	// 初始化扫描器和处理器
	scanner := services.NewMusicScanner(
		cfg.Music.Directory,
		cfg.Music.SupportedFormats,
		cfg.Music.CacheTTLMinutes,
	)

	playlistHandler := handlers.NewPlaylistHandler(scanner)
	streamHandler := handlers.NewStreamHandler(scanner, cfg)

	// 设置路由
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": "zero music服务器正在运行",
		})
	})

	api := router.Group("/api")
	{
		api.GET("/songs", playlistHandler.GetAllSongs)
		api.GET("/song/:id", playlistHandler.GetSongByID)
		api.GET("/stream/:id", streamHandler.StreamAudio)
	}

	return router, musicDir
}

// TestHealthCheck 测试健康检查端点
func TestHealthCheck(t *testing.T) {
	router, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际得到 %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if status, ok := response["status"].(string); !ok || status != "ok" {
		t.Errorf("期望状态为 'ok'，实际得到 '%v'", response["status"])
	}
}

// TestGetAllSongs 测试获取所有歌曲列表
func TestGetAllSongs(t *testing.T) {
	router, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/songs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际得到 %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if total, ok := response["total"].(float64); !ok || total < 1 {
		t.Errorf("期望至少有 1 首歌曲，实际得到 %v", response["total"])
	}

	// 检查是否包含 X-Request-ID 头
	if requestID := w.Header().Get("X-Request-ID"); requestID == "" {
		t.Error("响应缺少 X-Request-ID 头")
	}
}

// TestGetSongByInvalidID 测试使用无效 ID 获取歌曲
func TestGetSongByInvalidID(t *testing.T) {
	router, _ := setupTestServer(t)

	testCases := []struct {
		name       string
		id         string
		expectCode int
	}{
		{
			name:       "过短的 ID",
			id:         "abc123",
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "包含大写字母的 ID",
			id:         "ABCDEF1234567890ABCDEF1234567890",
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "包含非法字符的 ID",
			id:         "g0000000000000000000000000000000",
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/song/"+tc.id, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tc.expectCode {
				t.Errorf("期望状态码 %d，实际得到 %d", tc.expectCode, w.Code)
			}
		})
	}
}

// TestStreamAudioNotFound 测试流式传输不存在的音频
func TestStreamAudioNotFound(t *testing.T) {
	router, _ := setupTestServer(t)

	// 使用一个有效格式但不存在的 ID
	nonExistentID := "00000000000000000000000000000000"
	req := httptest.NewRequest(http.MethodGet, "/api/stream/"+nonExistentID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 %d，实际得到 %d", http.StatusNotFound, w.Code)
	}
}

// TestRequestIDMiddleware 测试请求 ID 中间件
func TestRequestIDMiddleware(t *testing.T) {
	router, _ := setupTestServer(t)

	t.Run("自动生成请求ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Error("响应缺少自动生成的 X-Request-ID 头")
		}
		if len(requestID) != 32 {
			t.Errorf("请求 ID 长度应为 32，实际为 %d", len(requestID))
		}
	})

	t.Run("保留客户端提供的请求ID", func(t *testing.T) {
		clientRequestID := "12345678901234567890123456789012"
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("X-Request-ID", clientRequestID)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		serverRequestID := w.Header().Get("X-Request-ID")
		if serverRequestID != clientRequestID {
			t.Errorf("期望请求 ID %s，实际得到 %s", clientRequestID, serverRequestID)
		}
	})
}

// TestConcurrentRequests 测试并发请求
func TestConcurrentRequests(t *testing.T) {
	router, _ := setupTestServer(t)

	const numRequests = 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/api/songs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("并发请求失败，状态码: %d", w.Code)
			}
			done <- true
		}()
	}

	// 等待所有请求完成
	for i := 0; i < numRequests; i++ {
		<-done
	}
}

// TestStreamAudioContent 测试流式传输音频内容
func TestStreamAudioContent(t *testing.T) {
	router, _ := setupTestServer(t)

	// 首先获取歌曲列表
	req := httptest.NewRequest(http.MethodGet, "/api/songs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	songs, ok := response["songs"].([]interface{})
	if !ok || len(songs) == 0 {
		t.Fatal("没有找到歌曲")
	}

	// 获取第一首歌的 ID
	firstSong := songs[0].(map[string]interface{})
	songID := firstSong["id"].(string)

	// 请求流式传输
	streamReq := httptest.NewRequest(http.MethodGet, "/api/stream/"+songID, nil)
	streamW := httptest.NewRecorder()
	router.ServeHTTP(streamW, streamReq)

	if streamW.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际得到 %d", http.StatusOK, streamW.Code)
	}

	// 验证内容类型
	contentType := streamW.Header().Get("Content-Type")
	if contentType == "" {
		t.Error("响应缺少 Content-Type 头")
	}

	// 验证内容
	body, err := io.ReadAll(streamW.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	if len(body) == 0 {
		t.Error("响应体为空")
	}
}
