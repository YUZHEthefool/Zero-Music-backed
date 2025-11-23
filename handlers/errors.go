package handlers

import (
	"fmt"
	"os"
)

// APIError 定义了 API 返回的标准化错误结构。
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Error 实现了标准错误接口。
func (e *APIError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewNotFoundError 创建一个表示资源未找到的 APIError。
func NewNotFoundError(resource string) *APIError {
	return &APIError{
		Code:    "NOT_FOUND",
		Message: fmt.Sprintf("%s未找到", resource),
	}
}

// NewInternalError 创建一个表示内部服务器错误的 APIError。
// 在生产环境中（ZERO_MUSIC_ENV=production），不会暴露错误详情。
func NewInternalError(err error) *APIError {
	apiErr := &APIError{
		Code:    "INTERNAL_ERROR",
		Message: "内部服务器错误",
	}
	
	// 仅在非生产环境中暴露错误详情
	if os.Getenv("ZERO_MUSIC_ENV") != "production" {
		apiErr.Details = err.Error()
	}
	
	return apiErr
}

// NewBadRequestError 创建一个表示无效请求的 APIError。
func NewBadRequestError(message string) *APIError {
	return &APIError{
		Code:    "BAD_REQUEST",
		Message: message,
	}
}

// NewForbiddenError 创建一个表示禁止访问的 APIError。
func NewForbiddenError(message string) *APIError {
	return &APIError{
		Code:    "FORBIDDEN",
		Message: message,
	}
}
