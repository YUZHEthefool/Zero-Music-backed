package services

import (
	"context"
	"zero-music/models"
)

// Scanner 定义了音乐扫描器的接口。
// 该接口提供了扫描音乐文件和管理歌曲列表缓存的抽象。
type Scanner interface {
	// Scan 扫描音乐目录并返回歌曲列表。
	// 为了提高性能，实现应该缓存扫描结果。
	Scan(ctx context.Context) ([]*models.Song, error)

	// Refresh 强制执行一次新的扫描，并刷新歌曲列表缓存。
	Refresh(ctx context.Context) error

	// GetSongs 返回当前缓存的歌曲列表。
	GetSongs() []*models.Song

	// GetSongCount 返回当前缓存的歌曲数量。
	GetSongCount() int

	// GetSongByID 根据 ID 查找并返回指定的歌曲。
	// 如果未找到歌曲，则返回 nil。
	GetSongByID(id string) *models.Song
}
