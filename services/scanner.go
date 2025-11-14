package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"zero-music/models"
)

// MusicScanner 负责扫描音乐目录并管理歌曲列表缓存。
type MusicScanner struct {
	directory        string
	supportedFormats []string
	songs            []*models.Song
	mu               sync.RWMutex
	lastScan         time.Time
	cacheTTL         time.Duration
}

// NewMusicScanner 创建并返回一个新的 MusicScanner 实例。
func NewMusicScanner(directory string, supportedFormats []string, cacheTTLMinutes int) *MusicScanner {
	if len(supportedFormats) == 0 {
		supportedFormats = []string{".mp3"}
	}
	if cacheTTLMinutes <= 0 {
		cacheTTLMinutes = 5
	}
	return &MusicScanner{
		directory:        directory,
		supportedFormats: supportedFormats,
		songs:            make([]*models.Song, 0),
		cacheTTL:         time.Duration(cacheTTLMinutes) * time.Minute,
	}
}

// Scan 扫描音乐目录并返回歌曲列表。
// 为了提高性能，此函数会缓存扫描结果。
// 如果缓存有效，它将返回缓存的数据；否则，它将执行新的扫描。
func (s *MusicScanner) Scan() ([]*models.Song, error) {
	s.mu.RLock()
	// 检查缓存是否仍然有效。
	if time.Since(s.lastScan) < s.cacheTTL && len(s.songs) > 0 {
		songs := make([]*models.Song, len(s.songs))
		copy(songs, s.songs)
		s.mu.RUnlock()
		return songs, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 在获取写锁后再次检查缓存，以避免在等待锁期间其他 goroutine 已刷新缓存。
	if time.Since(s.lastScan) < s.cacheTTL && len(s.songs) > 0 {
		songs := make([]*models.Song, len(s.songs))
		copy(songs, s.songs)
		return songs, nil
	}

	// 执行实际的扫描操作。
	return s.scanInternal()
}

// scanInternal 是实际的扫描逻辑。
// 调用此函数前必须获取写锁。
func (s *MusicScanner) scanInternal() ([]*models.Song, error) {
	s.songs = make([]*models.Song, 0)

	// 确保音乐目录存在。
	if _, err := os.Stat(s.directory); os.IsNotExist(err) {
		return nil, fmt.Errorf("音乐目录不存在: %s", s.directory)
	}

	// 遍历目录下的所有文件。
	err := filepath.Walk(s.directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 忽略目录。
		if info.IsDir() {
			return nil
		}

		// 检查文件扩展名是否受支持。
		ext := strings.ToLower(filepath.Ext(path))
		for _, supported := range s.supportedFormats {
			if ext == strings.ToLower(supported) {
				song := models.NewSong(path, info.Size())
				s.songs = append(s.songs, song)
				break
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("扫描目录时出错: %v", err)
	}

	s.lastScan = time.Now()
	return s.songs, nil
}

// Refresh 强制执行一次新的扫描，并刷新歌曲列表缓存。
func (s *MusicScanner) Refresh() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.scanInternal()
	return err
}

// GetSongs 返回当前缓存的歌曲列表的副本。
func (s *MusicScanner) GetSongs() []*models.Song {
	s.mu.RLock()
	defer s.mu.RUnlock()
	songs := make([]*models.Song, len(s.songs))
	copy(songs, s.songs)
	return songs
}

// GetSongCount 返回当前缓存的歌曲数量。
func (s *MusicScanner) GetSongCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.songs)
}
