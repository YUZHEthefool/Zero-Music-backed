package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"zero-music/config"
	"zero-music/services"

	"github.com/gin-gonic/gin"
)

const (
	// MaxRangeSize 定义了单次 Range 请求允许传输的最大字节数 (100MB)。
	MaxRangeSize = 100 * 1024 * 1024
)

// StreamHandler 负责处理音频流相关的 API 请求。
type StreamHandler struct {
	scanner     *services.MusicScanner
	musicDir    string
	musicDirAbs string // 预先计算的音乐目录绝对路径，用于安全检查。
}

// NewStreamHandler 创建一个新的 StreamHandler 实例。
func NewStreamHandler(cfg *config.Config) *StreamHandler {
	scanner := services.NewMusicScanner(
		cfg.Music.Directory,
		cfg.Music.SupportedFormats,
		cfg.Music.CacheTTLMinutes,
	)
	musicDirAbs, err := filepath.Abs(cfg.Music.Directory)
	if err != nil {
		log.Printf("警告: 获取音乐目录的绝对路径失败: %v", err)
		musicDirAbs = cfg.Music.Directory
	}
	return &StreamHandler{
		scanner:     scanner,
		musicDir:    cfg.Music.Directory,
		musicDirAbs: musicDirAbs,
	}
}

// StreamAudio 处理流式传输音频文件的请求。
// 它支持完整的音频文件传输和基于 Range 请求的部分内容传输。
// @Summary 流式传输音频
// @Description 通过 HTTP 流式传输指定的音频文件
// @Tags stream
// @Produce audio/mpeg
// @Param id path string true "歌曲ID"
// @Success 200 {file} binary "音频流"
// @Success 206 {file} binary "音频流(部分内容)"
// @Failure 400 {object} APIError "请求参数错误"
// @Failure 403 {object} APIError "禁止访问"
// @Failure 404 {object} APIError "文件未找到"
// @Failure 500 {object} APIError "服务器错误"
// @Router /api/stream/{id} [get]
func (h *StreamHandler) StreamAudio(c *gin.Context) {
	id := c.Param("id")

	// 验证 ID 格式，防止路径遍历攻击。
	if id == "" || strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		c.JSON(http.StatusBadRequest, NewBadRequestError("无效的歌曲 ID 格式"))
		return
	}

	// 扫描音乐文件以验证歌曲是否存在。
	songs, err := h.scanner.Scan()
	if err != nil {
		log.Printf("扫描音乐文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, NewInternalError(err))
		return
	}

	// 查找歌曲并获取其文件路径。
	var songPath string
	found := false
	for _, song := range songs {
		if song.ID == id {
			songPath = song.FilePath
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, NewNotFoundError("歌曲"))
		return
	}

	// 验证文件路径的安全性。
	cleanPath, err := filepath.Abs(songPath)
	if err != nil {
		log.Printf("获取文件绝对路径失败 %s: %v", songPath, err)
		c.JSON(http.StatusInternalServerError, NewInternalError(err))
		return
	}

	// 确保请求的路径位于配置的音乐目录内。
	if !strings.HasPrefix(cleanPath, h.musicDirAbs) {
		log.Printf("安全警告: 拒绝访问 - 路径 %s 不在音乐目录 %s 内", cleanPath, h.musicDirAbs)
		c.JSON(http.StatusForbidden, NewForbiddenError("拒绝访问"))
		return
	}

	// 检查文件是否存在。
	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, NewNotFoundError("音频文件"))
		} else {
			log.Printf("无法获取文件信息 %s: %v", cleanPath, err)
			c.JSON(http.StatusInternalServerError, NewInternalError(err))
		}
		return
	}

	// 确保请求的不是一个目录。
	if fileInfo.IsDir() {
		log.Printf("安全警告: 尝试流式传输目录: %s", cleanPath)
		c.JSON(http.StatusForbidden, NewForbiddenError("无法流式传输目录"))
		return
	}

	// 打开音频文件。
	file, err := os.Open(cleanPath)
	if err != nil {
		log.Printf("打开音频文件失败 %s: %v", cleanPath, err)
		c.JSON(http.StatusInternalServerError, NewInternalError(err))
		return
	}
	defer file.Close()

	fileSize := fileInfo.Size()

	// 记录访问日志。
	log.Printf("音频流请求: id=%s, path=%s, ip=%s, size=%d", id, cleanPath, c.ClientIP(), fileSize)

	// 处理 Range 请求以支持断点续传。
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		h.serveRange(c, file, fileSize, rangeHeader, filepath.Base(cleanPath))
		return
	}

	// 为完整文件传输设置响应头。
	c.Header("Content-Type", "audio/mpeg")
	c.Header("Content-Length", fmt.Sprintf("%d", fileSize))
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filepath.Base(cleanPath)))
	c.Header("Accept-Ranges", "bytes")

	// 流式传输整个文件。
	c.Status(http.StatusOK)
	written, err := io.Copy(c.Writer, file)
	if err != nil {
		log.Printf("流式传输音频时出错 (已写入 %d/%d 字节): %v", written, fileSize, err)
	}
}

// serveRange 处理 HTTP Range 请求，用于支持音频的断点续传。
func (h *StreamHandler) serveRange(c *gin.Context, file *os.File, fileSize int64, rangeHeader string, filename string) {
	ranges := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(ranges, "-")

	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, NewBadRequestError("无效的 Range 请求头格式"))
		return
	}

	start := int64(0)
	end := fileSize - 1

	// 解析范围的起始位置。
	if parts[0] != "" {
		var err error
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 {
			c.JSON(http.StatusBadRequest, NewBadRequestError("无效的 Range 起始值"))
			return
		}
	}

	// 解析范围的结束位置。
	if parts[1] != "" {
		var err error
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < 0 {
			c.JSON(http.StatusBadRequest, NewBadRequestError("无效的 Range 结束值"))
			return
		}
	}

	// 验证请求范围的有效性。
	if start < 0 || end >= fileSize || start > end {
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	contentLength := end - start + 1

	// 限制单次请求的数据大小。
	if contentLength > MaxRangeSize {
		log.Printf("Range 请求过大: %d 字节 (最大 %d)", contentLength, MaxRangeSize)
		c.JSON(http.StatusBadRequest, NewBadRequestError(fmt.Sprintf("请求范围过大 (最大 %d 字节)", MaxRangeSize)))
		return
	}

	// 设置部分内容响应的头部。
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Header("Content-Length", fmt.Sprintf("%d", contentLength))
	c.Header("Content-Type", "audio/mpeg")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
	c.Header("Accept-Ranges", "bytes")
	c.Status(http.StatusPartialContent)

	// 将文件指针移动到请求的起始位置。
	_, err := file.Seek(start, 0)
	if err != nil {
		log.Printf("定位文件到 %d 位置失败: %v", start, err)
		c.JSON(http.StatusInternalServerError, NewInternalError(err))
		return
	}

	// 传输指定范围的数据。
	written, err := io.CopyN(c.Writer, file, contentLength)
	if err != nil && err != io.EOF {
		log.Printf("流式传输范围时出错 (已写入 %d/%d 字节): %v", written, contentLength, err)
	}
}
