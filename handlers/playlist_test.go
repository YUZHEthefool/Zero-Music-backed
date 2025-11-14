package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"zero-music/config"

	"github.com/gin-gonic/gin"
)

// setupTestEnv 初始化一个用于播放列表处理器测试的环境。
// 它会创建一个临时的音乐目录和一些测试文件，并返回一个配置好的 Gin 引擎。
func setupTestEnv(t *testing.T) (*gin.Engine, string) {
	// 将 Gin 设置为测试模式。
	gin.SetMode(gin.TestMode)

	// 创建一个临时目录来存放测试音乐文件。
	tmpDir := t.TempDir()

	// 创建假的 MP3 文件用于测试。
	testFile1 := filepath.Join(tmpDir, "test1.mp3")
	testFile2 := filepath.Join(tmpDir, "test2.mp3")
	if err := os.WriteFile(testFile1, []byte("fake mp3 data 1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile2, []byte("fake mp3 data 2"), 0644); err != nil {
		t.Fatal(err)
	}

	// 使用临时目录创建测试配置。
	cfg := &config.Config{
		Music: config.MusicConfig{
			Directory:        tmpDir,
			SupportedFormats: []string{".mp3"},
			CacheTTLMinutes:  5,
		},
	}

	// 创建 Gin 路由器并注册处理器。
	router := gin.New()
	handler := NewPlaylistHandler(cfg)
	router.GET("/api/songs", handler.GetAllSongs)
	router.GET("/api/song/:id", handler.GetSongByID)

	return router, tmpDir
}

// TestGetAllSongs 测试 GetAllSongs 端点是否能成功返回所有歌曲。
func TestGetAllSongs(t *testing.T) {
	router, _ := setupTestEnv(t)

	// 创建一个 HTTP 请求来测试端点。
	req, _ := http.NewRequest("GET", "/api/songs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证 HTTP 状态码是否为 200 OK。
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 得到 %d", w.Code)
	}

	// 解析 JSON 响应。
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证响应中歌曲的总数是否正确。
	total, ok := response["total"].(float64)
	if !ok {
		t.Fatal("响应中缺少 'total' 字段")
	}
	if total != 2 {
		t.Errorf("期望有 2 首歌曲, 得到 %v", total)
	}

	// 验证响应中是否存在 'songs' 字段。
	if _, ok := response["songs"]; !ok {
		t.Error("响应中缺少 'songs' 字段")
	}
}

// TestGetSongByID_Success 测试 GetSongByID 端点在找到歌曲时是否能成功返回。
func TestGetSongByID_Success(t *testing.T) {
	router, _ := setupTestEnv(t)

	// 首先调用 /api/songs 获取一个有效的歌曲 ID。
	req, _ := http.NewRequest("GET", "/api/songs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	songs := response["songs"].([]interface{})
	firstSong := songs[0].(map[string]interface{})
	songID := firstSong["id"].(string)

	// 使用获取到的 ID 请求特定的歌曲。
	req, _ = http.NewRequest("GET", "/api/song/"+songID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 得到 %d", w.Code)
	}

	var song map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &song); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if song["id"] != songID {
		t.Errorf("期望的歌曲 ID 是 %s, 得到 %v", songID, song["id"])
	}
}

// TestGetSongByID_NotFound 测试 GetSongByID 端点在歌曲未找到时是否返回 404。
func TestGetSongByID_NotFound(t *testing.T) {
	router, _ := setupTestEnv(t)

	// 请求一个不存在的歌曲 ID。
	req, _ := http.NewRequest("GET", "/api/song/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 404, 得到 %d", w.Code)
	}
}

// TestGetSongByID_InvalidFormat 测试 GetSongByID 端点对无效的 ID 格式是否能正确处理。
func TestGetSongByID_InvalidFormat(t *testing.T) {
	router, _ := setupTestEnv(t)

	testCases := []struct {
		name string
		id   string
	}{
		{"包含 '..' 的路径遍历", "../etc/passwd"},
		{"包含 '/' 的路径", "path/to/file"},
		{"包含 '\\' 的路径", "path\\to\\file"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/song/"+tc.id, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// 对于无效格式，我们期望返回 400 Bad Request。
			// 404 也是可接受的，因为即使验证通过，歌曲也找不到。
			if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
				t.Errorf("对于 %s，期望状态码 400 或 404, 得到 %d", tc.name, w.Code)
			}
		})
	}
}
