package models

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Song 定义了歌曲的基本信息结构。
type Song struct {
	// ID 是歌曲的唯一标识符，通过文件路径的 SHA256 哈希生成。
	ID string `json:"id"`
	// Title 是歌曲的标题，通常从文件名中提取。
	Title string `json:"title"`
	// Artist 是歌曲的艺术家，默认为 "Unknown"。
	Artist string `json:"artist"`
	// Album 是歌曲所属的专辑，默认为 "Unknown"。
	Album string `json:"album"`
	// Duration 是歌曲的时长（以秒为单位），默认为 0。
	Duration int `json:"duration"`
	// FilePath 是歌曲文件的绝对路径。
	FilePath string `json:"file_path"`
	// FileName 是歌曲的文件名。
	FileName string `json:"file_name"`
	// FileSize 是歌曲文件的大小（以字节为单位）。
	FileSize int64 `json:"file_size"`
	// AddedAt 是歌曲文件最后修改的时间。
	AddedAt time.Time `json:"added_at"`
}

// NewSong 根据给定的文件路径和文件大小创建一个新的 Song 实例。
func NewSong(filePath string, fileSize int64) *Song {
	fileName := filepath.Base(filePath)
	// 默认使用移除了扩展名的文件名作为标题。
	title := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// 使用文件的修改时间作为添加时间。
	addedAt := time.Now()
	if info, err := os.Stat(filePath); err == nil {
		addedAt = info.ModTime()
	}

	return &Song{
		ID:       generateID(filePath),
		Title:    title,
		Artist:   "Unknown", // TODO: 将来可以实现从 ID3 标签读取
		Album:    "Unknown", // TODO: 将来可以实现从 ID3 标签读取
		Duration: 0,       // TODO: 将来可以实现从媒体文件头读取
		FilePath: filePath,
		FileName: fileName,
		FileSize: fileSize,
		AddedAt:  addedAt,
	}
}

// generateID 使用文件路径的 SHA256 哈希值的前 16 字节生成一个唯一的歌曲 ID。
func generateID(filePath string) string {
	hash := sha256.Sum256([]byte(filePath))
	return hex.EncodeToString(hash[:16])
}
