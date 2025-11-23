package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"time"
	"zero-music/logger"

	"github.com/gin-gonic/gin"
)

const (
	// RequestIDHeader HTTP 头部中的请求 ID 字段名
	RequestIDHeader = "X-Request-ID"
	// RequestIDKey 在 Gin Context 中存储请求 ID 的键名
	RequestIDKey = "request_id"
	// RequestIDByteLength 请求 ID 的字节长度（生成 32 个十六进制字符）
	RequestIDByteLength = 16
)

// generateRequestID 生成一个唯一的请求 ID
func generateRequestID() string {
	b := make([]byte, RequestIDByteLength)
	if _, err := rand.Read(b); err != nil {
		// 如果随机数生成失败，使用时间戳作为备选方案
		return hex.EncodeToString([]byte(time.Now().String()))[:32]
	}
	return hex.EncodeToString(b)
}

// RequestID 是一个 Gin 中间件，为每个请求生成唯一 ID
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 尝试从请求头获取现有的请求 ID
		requestID := c.GetHeader(RequestIDHeader)

		// 如果没有请求 ID，则生成一个新的
		if requestID == "" {
			requestID = generateRequestID()
		}

		// 将请求 ID 存储在上下文中
		c.Set(RequestIDKey, requestID)

		// 在响应头中添加请求 ID
		c.Header(RequestIDHeader, requestID)

		// 记录请求开始
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		logger.WithRequestID(requestID).WithFields(map[string]interface{}{
			"method":     method,
			"path":       path,
			"client_ip":  c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		}).Info("请求开始")

		// 继续处理请求
		c.Next()

		// 记录请求完成
		latency := time.Since(start)
		status := c.Writer.Status()

		logEntry := logger.WithRequestID(requestID).WithFields(map[string]interface{}{
			"method":     method,
			"path":       path,
			"status":     status,
			"latency_ms": latency.Milliseconds(),
			"client_ip":  c.ClientIP(),
		})

		if status >= 500 {
			logEntry.Error("请求完成（服务器错误）")
		} else if status >= 400 {
			logEntry.Warn("请求完成（客户端错误）")
		} else {
			logEntry.Info("请求完成")
		}
	}
}

// GetRequestID 从 Gin Context 中获取请求 ID
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get(RequestIDKey); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}
