package handlers

import (
	"log"
	"net/http"
	"strings"
	"zero-music/config"
	"zero-music/services"

	"github.com/gin-gonic/gin"
)

// PlaylistHandler 负责处理与播放列表相关的 API 请求。
type PlaylistHandler struct {
	scanner *services.MusicScanner
}

// NewPlaylistHandler 创建一个新的 PlaylistHandler 实例。
func NewPlaylistHandler(cfg *config.Config) *PlaylistHandler {
	scanner := services.NewMusicScanner(
		cfg.Music.Directory,
		cfg.Music.SupportedFormats,
		cfg.Music.CacheTTLMinutes,
	)
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
	// 扫描音乐文件。
	songs, err := h.scanner.Scan()
	if err != nil {
		log.Printf("扫描音乐文件失败: %v", err)
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

	// 验证 ID 格式，防止路径遍历。
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		c.JSON(http.StatusBadRequest, NewBadRequestError("无效的歌曲 ID 格式"))
		return
	}

	// 扫描以获取所有歌曲。
	songs, err := h.scanner.Scan()
	if err != nil {
		log.Printf("扫描音乐文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, NewInternalError(err))
		return
	}

	// 查找具有指定 ID 的歌曲。
	for _, song := range songs {
		if song.ID == id {
			c.JSON(http.StatusOK, song)
			return
		}
	}

	// 如果未找到歌曲，则返回 404 错误。
	c.JSON(http.StatusNotFound, NewNotFoundError("歌曲"))
}
