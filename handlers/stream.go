package handlers

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"zero-music/config"
	"zero-music/logger"
	"zero-music/middleware"
	"zero-music/models"
	"zero-music/services"

	"github.com/gin-gonic/gin"
)

var (
	// validIDPatternStream 验证歌曲 ID 是否为有效的 SHA256 哈希（32 字节十六进制）
	validIDPatternStream = regexp.MustCompile(models.ValidIDPattern())
)

// getMimeType 根据文件扩展名返回对应的 MIME 类型。
func getMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		// 为常见音频格式提供备选 MIME 类型
		switch ext {
		case ".mp3":
			return "audio/mpeg"
		case ".flac":
			return "audio/flac"
		case ".wav":
			return "audio/wav"
		case ".m4a":
			return "audio/mp4"
		case ".ogg":
			return "audio/ogg"
		default:
			return "application/octet-stream"
		}
	}
	return mimeType
}

// StreamHandler 负责处理音频流相关的 API 请求。
type StreamHandler struct {
	scanner      services.Scanner
	musicDir     string
	musicDirAbs  string // 预先计算的音乐目录绝对路径，用于安全检查。
	maxRangeSize int64  // 单次 Range 请求允许的最大字节数。
}

// NewStreamHandler 创建一个新的 StreamHandler 实例。
func NewStreamHandler(scanner services.Scanner, cfg *config.Config) *StreamHandler {
	musicDirAbs, err := filepath.Abs(cfg.Music.Directory)
	if err != nil {
		logger.Warnf("获取音乐目录的绝对路径失败: %v", err)
		musicDirAbs = cfg.Music.Directory
	}
	return &StreamHandler{
		scanner:      scanner,
		musicDir:     cfg.Music.Directory,
		musicDirAbs:  musicDirAbs,
		maxRangeSize: cfg.Server.MaxRangeSize,
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
	requestID := middleware.GetRequestID(c)

	// 验证 ID 格式，确保是有效的 SHA256 哈希格式，防止路径遍历攻击。
	if !validIDPatternStream.MatchString(id) {
		logger.WithRequestID(requestID).Warnf("无效的歌曲 ID 格式: %s", id)
		c.JSON(http.StatusBadRequest, NewBadRequestError("无效的歌曲 ID 格式"))
		return
	}

	// 扫描音乐文件以验证歌曲是否存在。
	songs, err := h.scanner.Scan(c.Request.Context())
	if err != nil {
		logger.WithRequestID(requestID).Errorf("扫描音乐文件失败: %v", err)
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
		logger.WithRequestID(requestID).Warnf("歌曲未找到: %s", id)
		c.JSON(http.StatusNotFound, NewNotFoundError("歌曲"))
		return
	}

	// 验证文件路径的安全性。
	cleanPath, err := filepath.Abs(songPath)
	if err != nil {
		logger.WithRequestID(requestID).Errorf("获取文件绝对路径失败 %s: %v", songPath, err)
		c.JSON(http.StatusInternalServerError, NewInternalError(err))
		return
	}

	// 确保请求的路径位于配置的音乐目录内。
	if !strings.HasPrefix(cleanPath, h.musicDirAbs) {
		logger.WithRequestID(requestID).Warnf("安全警告: 拒绝访问 - 路径 %s 不在音乐目录 %s 内", cleanPath, h.musicDirAbs)
		c.JSON(http.StatusForbidden, NewForbiddenError("拒绝访问"))
		return
	}

	// 检查文件是否存在。
	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, NewNotFoundError("音频文件"))
		} else {
			logger.WithRequestID(requestID).Errorf("无法获取文件信息 %s: %v", cleanPath, err)
			c.JSON(http.StatusInternalServerError, NewInternalError(err))
		}
		return
	}

	// 确保请求的不是一个目录。
	if fileInfo.IsDir() {
		logger.WithRequestID(requestID).Warnf("安全警告: 尝试流式传输目录: %s", cleanPath)
		c.JSON(http.StatusForbidden, NewForbiddenError("无法流式传输目录"))
		return
	}

	// 打开音频文件。
	file, err := os.Open(cleanPath)
	if err != nil {
		logger.WithRequestID(requestID).Errorf("打开音频文件失败 %s: %v", cleanPath, err)
		c.JSON(http.StatusInternalServerError, NewInternalError(err))
		return
	}
	defer file.Close()

	fileSize := fileInfo.Size()

	// 记录访问日志。
	logger.WithRequestID(requestID).WithFields(map[string]interface{}{
		"song_id":   id,
		"file_path": cleanPath,
		"file_size": fileSize,
	}).Info("音频流请求")

	// 处理 Range 请求以支持断点续传。
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		h.serveRange(c, file, fileSize, rangeHeader, filepath.Base(cleanPath), requestID)
		return
	}

	// 为完整文件传输设置响应头。
	mimeType := getMimeType(cleanPath)
	c.Header("Content-Type", mimeType)
	c.Header("Content-Length", fmt.Sprintf("%d", fileSize))
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filepath.Base(cleanPath)))
	c.Header("Accept-Ranges", "bytes")

	// 流式传输整个文件。
	c.Status(http.StatusOK)
	written, err := io.Copy(c.Writer, file)
	if err != nil {
		logger.WithRequestID(requestID).Errorf("流式传输音频时出错 (已写入 %d/%d 字节): %v", written, fileSize, err)
	}
}

// serveRange 处理 HTTP Range 请求，用于支持音频的断点续传。
func (h *StreamHandler) serveRange(c *gin.Context, file *os.File, fileSize int64, rangeHeader string, filename string, requestID string) {
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
	if contentLength > h.maxRangeSize {
		logger.WithRequestID(requestID).Warnf("Range 请求过大: %d 字节 (最大 %d)", contentLength, h.maxRangeSize)
		c.JSON(http.StatusBadRequest, NewBadRequestError(fmt.Sprintf("请求范围过大 (最大 %d 字节)", h.maxRangeSize)))
		return
	}

	// 设置部分内容响应的头部。
	mimeType := getMimeType(filename)
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Header("Content-Length", fmt.Sprintf("%d", contentLength))
	c.Header("Content-Type", mimeType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
	c.Header("Accept-Ranges", "bytes")
	c.Status(http.StatusPartialContent)

	// 将文件指针移动到请求的起始位置。
	_, err := file.Seek(start, 0)
	if err != nil {
		logger.WithRequestID(requestID).Errorf("定位文件到 %d 位置失败: %v", start, err)
		c.JSON(http.StatusInternalServerError, NewInternalError(err))
		return
	}

	// 传输指定范围的数据。
	written, err := io.CopyN(c.Writer, file, contentLength)
	if err != nil && err != io.EOF {
		logger.WithRequestID(requestID).Errorf("流式传输范围时出错 (已写入 %d/%d 字节): %v", written, contentLength, err)
	}
}
