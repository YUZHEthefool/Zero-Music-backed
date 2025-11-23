package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"zero-music/models"
)

// MusicScanner 负责扫描音乐目录并管理歌曲列表缓存。
// 它实现了 Scanner 接口。
type MusicScanner struct {
	directory        string
	supportedFormats []string
	songs            []*models.Song
	songIndex        map[string]*models.Song // ID -> Song 的索引，用于快速查找
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
		songIndex:        make(map[string]*models.Song),
		cacheTTL:         time.Duration(cacheTTLMinutes) * time.Minute,
	}
}

// Scan 扫描音乐目录并返回歌曲列表。
// 为了提高性能，此函数会缓存扫描结果。
// 如果缓存有效，它将返回缓存的数据；否则，它将执行新的扫描。
func (s *MusicScanner) Scan(ctx context.Context) ([]*models.Song, error) {
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
	return s.scanInternal(ctx)
}

// scanInternal 是实际的扫描逻辑。
// 调用此函数前必须获取写锁。
func (s *MusicScanner) scanInternal(ctx context.Context) ([]*models.Song, error) {
	s.songs = make([]*models.Song, 0)
	s.songIndex = make(map[string]*models.Song)

	// 确保音乐目录存在。
	if _, err := os.Stat(s.directory); os.IsNotExist(err) {
		return nil, fmt.Errorf("音乐目录不存在: %s", s.directory)
	}

	// 遍历目录下的所有文件。
	err := filepath.Walk(s.directory, func(path string, info os.FileInfo, err error) error {
		// 检查 context 是否被取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

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
				s.songIndex[song.ID] = song
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

// Refresh 强制执行一次新的扫描,并刷新歌曲列表缓存。
func (s *MusicScanner) Refresh(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.scanInternal(ctx)
	return err
}

// GetSongs 返回当前缓存的歌曲列表的深度拷贝。
// 使用深度拷贝避免外部修改影响缓存数据。
func (s *MusicScanner) GetSongs() []*models.Song {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 创建深度拷贝
	songs := make([]*models.Song, len(s.songs))
	for i, song := range s.songs {
		if song != nil {
			// 拷贝 Song 结构体
			copiedSong := *song
			// 拷贝 SupportedFormats 切片（如果 Song 中有的话）
			songs[i] = &copiedSong
		}
	}
	return songs
}

// GetSongCount 返回当前缓存的歌曲数量。
func (s *MusicScanner) GetSongCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.songs)
}

// GetSongByID 根据 ID 查找并返回指定的歌曲。
// 如果未找到歌曲，则返回 nil。
// 此方法使用索引进行高效查找。
func (s *MusicScanner) GetSongByID(id string) *models.Song {
	s.mu.RLock()
	defer s.mu.RUnlock()
	song, ok := s.songIndex[id]
	if !ok || song == nil {
		return nil
	}
	copiedSong := *song
	return &copiedSong
}
