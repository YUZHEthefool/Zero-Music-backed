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

// setupStreamTestEnv 初始化一个用于音频流处理器测试的环境。
// 它会创建一个临时的 MP3 文件并设置好 Gin 路由器。
func setupStreamTestEnv(t *testing.T) (*gin.Engine, string, string) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp3")
	testData := []byte("fake mp3 data for streaming test")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Music: config.MusicConfig{
			Directory:        tmpDir,
			SupportedFormats: []string{".mp3"},
			CacheTTLMinutes:  5,
		},
	}

	router := gin.New()
	handler := NewStreamHandler(cfg)

	// 为了获取歌曲 ID，我们需要一个播放列表端点。
	playlistHandler := NewPlaylistHandler(cfg)
	router.GET("/api/songs", playlistHandler.GetAllSongs)
	router.GET("/api/stream/:id", handler.StreamAudio)

	return router, tmpDir, testFile
}

// getSongID 是一个辅助函数，用于从 /api/songs 端点获取第一首歌曲的 ID。
func getSongID(t *testing.T, router *gin.Engine) string {
	req, _ := http.NewRequest("GET", "/api/songs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	songs := response["songs"].([]interface{})
	firstSong := songs[0].(map[string]interface{})
	return firstSong["id"].(string)
}

// TestStreamAudio_Success 测试音频流是否能成功传输。
func TestStreamAudio_Success(t *testing.T) {
	router, _, _ := setupStreamTestEnv(t)
	songID := getSongID(t, router)

	req, _ := http.NewRequest("GET", "/api/stream/"+songID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 得到 %d", w.Code)
	}

	// 检查响应头是否正确。
	if w.Header().Get("Content-Type") != "audio/mpeg" {
		t.Error("期望 Content-Type 为 audio/mpeg")
	}
	if w.Header().Get("Accept-Ranges") != "bytes" {
		t.Error("期望 Accept-Ranges 为 bytes")
	}

	// 检查响应体是否包含数据。
	if w.Body.Len() == 0 {
		t.Error("期望响应体不为空")
	}
}

// TestStreamAudio_NotFound 测试当请求一个不存在的歌曲 ID 时，是否返回 404。
func TestStreamAudio_NotFound(t *testing.T) {
	router, _, _ := setupStreamTestEnv(t)

	req, _ := http.NewRequest("GET", "/api/stream/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 404, 得到 %d", w.Code)
	}
}

// TestStreamAudio_InvalidID 测试当提供一个无效的歌曲 ID 时，是否返回错误。
func TestStreamAudio_InvalidID(t *testing.T) {
	router, _, _ := setupStreamTestEnv(t)

	testCases := []string{
		"../etc/passwd",
		"path/to/file",
		"path\\to\\file",
		"",
	}

	for _, id := range testCases {
		req, _ := http.NewRequest("GET", "/api/stream/"+id, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
			t.Errorf("对于 ID '%s'，期望状态码 400 或 404, 得到 %d", id, w.Code)
		}
	}
}

// TestStreamAudio_RangeRequest 测试是否正确处理 Range 请求。
func TestStreamAudio_RangeRequest(t *testing.T) {
	router, _, _ := setupStreamTestEnv(t)
	songID := getSongID(t, router)

	// 请求文件的前 10 个字节。
	req, _ := http.NewRequest("GET", "/api/stream/"+songID, nil)
	req.Header.Set("Range", "bytes=0-9")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("期望状态码 206, 得到 %d", w.Code)
	}

	// 检查 Content-Range 响应头。
	contentRange := w.Header().Get("Content-Range")
	if contentRange == "" {
		t.Error("期望包含 Content-Range 响应头")
	}

	// 检查响应体的大小是否为 10 字节。
	if w.Body.Len() != 10 {
		t.Errorf("期望响应体大小为 10 字节, 得到 %d", w.Body.Len())
	}
}

// TestStreamAudio_InvalidRange 测试当提供无效的 Range 请求头时，是否返回错误。
func TestStreamAudio_InvalidRange(t *testing.T) {
	router, _, _ := setupStreamTestEnv(t)
	songID := getSongID(t, router)

	testCases := []struct {
		name  string
		range_ string
	}{
		{"无效格式", "invalid"},
		{"负数起始", "bytes=-10-20"},
		{"无效字符", "bytes=abc-def"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/stream/"+songID, nil)
			req.Header.Set("Range", tc.range_)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest && w.Code != http.StatusRequestedRangeNotSatisfiable {
				t.Errorf("期望状态码 400 或 416, 得到 %d", w.Code)
			}
		})
	}
}
