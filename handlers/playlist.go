package handlers

import (
	"net/http"
	"regexp"
	"zero-music/logger"
	"zero-music/middleware"
	"zero-music/models"
	"zero-music/services"

	"github.com/gin-gonic/gin"
)

var (
	// validIDPattern 验证歌曲 ID 是否为有效的 SHA256 哈希（32 字节十六进制，即 64 个字符）
	// 注意：generateID 函数使用前 16 字节，因此是 32 个十六进制字符
	validIDPattern = regexp.MustCompile(models.ValidIDPattern())
)

// PlaylistHandler 负责处理与播放列表相关的 API 请求。
type PlaylistHandler struct {
	scanner services.Scanner
}

// NewPlaylistHandler 创建一个新的 PlaylistHandler 实例。
func NewPlaylistHandler(scanner services.Scanner) *PlaylistHandler {
	return &PlaylistHandler{
		scanner: scanner,
	}
}

// GetAllSongs 处理获取所有歌曲列表的请求。
// @Summary 获取所有歌曲
// @Description 返回音乐目录中所有可用的歌曲列表
// @Tags playlist
// @Produce json
// @Success 200 {object} map[string]interface{} "成功返回歌曲列表"
// @Failure 500 {object} APIError "服务器错误"
// @Router /api/songs [get]
func (h *PlaylistHandler) GetAllSongs(c *gin.Context) {
	requestID := middleware.GetRequestID(c)

	// 扫描音乐文件。
	songs, err := h.scanner.Scan(c.Request.Context())
	if err != nil {
		logger.WithRequestID(requestID).Errorf("扫描音乐文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, NewInternalError(err))
		return
	}

	// 返回歌曲列表。
	c.JSON(http.StatusOK, gin.H{
		"total": len(songs),
		"songs": songs,
	})
}

// GetSongByID 处理根据 ID 获取特定歌曲信息的请求。
// @Summary 获取指定歌曲信息
// @Description 根据歌曲ID返回歌曲详细信息
// @Tags playlist
// @Produce json
// @Param id path string true "歌曲ID"
// @Success 200 {object} models.Song "成功返回歌曲信息"
// @Failure 400 {object} APIError "请求参数错误"
// @Failure 404 {object} APIError "歌曲未找到"
// @Failure 500 {object} APIError "服务器错误"
// @Router /api/song/{id} [get]
func (h *PlaylistHandler) GetSongByID(c *gin.Context) {
	id := c.Param("id")
	requestID := middleware.GetRequestID(c)

	// 验证 ID 格式，确保是有效的 SHA256 哈希格式，防止路径遍历。
	if !validIDPattern.MatchString(id) {
		logger.WithRequestID(requestID).Warnf("无效的歌曲 ID 格式: %s", id)
		c.JSON(http.StatusBadRequest, NewBadRequestError("无效的歌曲 ID 格式"))
		return
	}

	// 先执行扫描以确保缓存是最新的。
	_, err := h.scanner.Scan(c.Request.Context())
	if err != nil {
		logger.WithRequestID(requestID).Errorf("扫描音乐文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, NewInternalError(err))
		return
	}

	// 使用索引快速查找歌曲。
	song := h.scanner.GetSongByID(id)
	if song == nil {
		logger.WithRequestID(requestID).Warnf("歌曲未找到: %s", id)
		c.JSON(http.StatusNotFound, NewNotFoundError("歌曲"))
		return
	}

	c.JSON(http.StatusOK, song)
}
