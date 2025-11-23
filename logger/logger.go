package logger

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

const (
	// DefaultLogLevel 是默认的日志级别
	DefaultLogLevel = "info"
)

var log *logrus.Logger

// Init 初始化日志系统
func Init(logFilePath string) (*os.File, error) {
	log = logrus.New()

	// 设置日志格式为 JSON，便于结构化处理
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// 打开日志文件
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Warnf("无法打开日志文件 %s: %v，日志将仅输出到标准输出", logFilePath, err)
		log.SetOutput(os.Stdout)
		return nil, err
	}

	// 同时输出到文件和标准输出
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	
	// 从环境变量读取日志级别，如果未设置则使用默认级别
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = DefaultLogLevel
	}
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		log.Warnf("无效的日志级别 '%s'，使用默认级别 '%s'", logLevel, DefaultLogLevel)
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	return logFile, nil
}

// GetLogger 返回全局日志实例
func GetLogger() *logrus.Logger {
	if log == nil {
		log = logrus.New()
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
		log.SetOutput(os.Stdout)
	}
	return log
}

// WithRequestID 创建带有请求 ID 的日志条目
func WithRequestID(requestID string) *logrus.Entry {
	return GetLogger().WithField("request_id", requestID)
}

// Info 记录信息级别日志
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Infof 格式化记录信息级别日志
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Warn 记录警告级别日志
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Warnf 格式化记录警告级别日志
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Error 记录错误级别日志
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Errorf 格式化记录错误级别日志
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Fatal 记录致命错误并退出
func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

// Fatalf 格式化记录致命错误并退出
func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}
