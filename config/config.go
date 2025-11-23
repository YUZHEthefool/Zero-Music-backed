package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const (
	// DefaultMaxRangeSize 是单次 Range 请求允许的默认最大字节数（100MB）
	DefaultMaxRangeSize = 100 * 1024 * 1024
	// DefaultCacheTTLMinutes 是音乐列表缓存的默认有效期（分钟）
	DefaultCacheTTLMinutes = 5
	// DefaultServerHost 是服务器的默认监听地址
	DefaultServerHost = "0.0.0.0"
	// DefaultServerPort 是服务器的默认监听端口
	DefaultServerPort = 8080

	// MaxAllowedRangeSize 是单次 Range 请求允许的最大字节数上限（500MB）
	MaxAllowedRangeSize = 500 * 1024 * 1024
	// MaxAllowedCacheTTL 是缓存 TTL 的最大允许值（分钟）
	MaxAllowedCacheTTL = 1440 // 24 hours
)

// Config 定义了应用程序的所有配置项。
type Config struct {
	Server ServerConfig `json:"server"`
	Music  MusicConfig  `json:"music"`
}

// ServerConfig 定义了服务器相关的配置。
type ServerConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	MaxRangeSize int64  `json:"max_range_size"` // 单次 Range 请求允许的最大字节数
}

// MusicConfig 定义了音乐库相关的配置。
type MusicConfig struct {
	// Directory 是音乐文件所在的目录。
	Directory string `json:"directory"`
	// SupportedFormats 是支持的音频文件格式列表。
	SupportedFormats []string `json:"supported_formats"`
	// CacheTTLMinutes 是音乐列表缓存的有效期（分钟）。
	CacheTTLMinutes int `json:"cache_ttl_minutes"`
}

// Load 从指定的路径加载配置文件。
// 如果 configPath 为空,则返回默认配置。
func Load(configPath string) (*Config, error) {
	if configPath == "" {
		return GetDefaultConfig(), nil
	}

	// 读取配置文件。
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 为空字段设置默认值。
	if len(cfg.Music.SupportedFormats) == 0 {
		cfg.Music.SupportedFormats = []string{".mp3", ".flac", ".wav", ".m4a", ".ogg"}
	}
	if cfg.Music.CacheTTLMinutes == 0 {
		cfg.Music.CacheTTLMinutes = DefaultCacheTTLMinutes
	}
	if cfg.Server.MaxRangeSize == 0 {
		cfg.Server.MaxRangeSize = DefaultMaxRangeSize
	}

	// 验证配置的有效性
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %v", err)
	}

	// 将音乐目录的相对路径转换为绝对路径。
	if !filepath.IsAbs(cfg.Music.Directory) {
		absPath, err := filepath.Abs(cfg.Music.Directory)
		if err == nil {
			cfg.Music.Directory = absPath
		}
	}

	// 应用环境变量覆盖配置
	applyEnvOverrides(&cfg)

	return &cfg, nil
}

// applyEnvOverrides 使用环境变量覆盖配置
func applyEnvOverrides(cfg *Config) {
	// 服务器配置
	if host := os.Getenv("ZERO_MUSIC_SERVER_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if port := os.Getenv("ZERO_MUSIC_SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil && p > 0 && p <= 65535 {
			cfg.Server.Port = p
		}
	}
	if maxRange := os.Getenv("ZERO_MUSIC_MAX_RANGE_SIZE"); maxRange != "" {
		if size, err := strconv.ParseInt(maxRange, 10, 64); err == nil && size > 0 && size <= MaxAllowedRangeSize {
			cfg.Server.MaxRangeSize = size
		}
	}

	// 音乐配置
	if musicDir := os.Getenv("ZERO_MUSIC_MUSIC_DIRECTORY"); musicDir != "" {
		if !filepath.IsAbs(musicDir) {
			if absPath, err := filepath.Abs(musicDir); err == nil {
				musicDir = absPath
			}
		}
		cfg.Music.Directory = musicDir
	}
	if cacheTTL := os.Getenv("ZERO_MUSIC_CACHE_TTL_MINUTES"); cacheTTL != "" {
		if ttl, err := strconv.Atoi(cacheTTL); err == nil && ttl > 0 && ttl <= MaxAllowedCacheTTL {
			cfg.Music.CacheTTLMinutes = ttl
		}
	}
}

// ProvideConfig 是 Wire 的提供者函数,用于加载配置
func ProvideConfig(configPath string) (*Config, error) {
	return Load(configPath)
}

// validateConfig 验证配置的合法性
func validateConfig(cfg *Config) error {
	// 验证端口范围
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("端口必须在 1-65535 范围内，当前值: %d", cfg.Server.Port)
	}

	// 验证 MaxRangeSize
	if cfg.Server.MaxRangeSize < 0 || cfg.Server.MaxRangeSize > MaxAllowedRangeSize {
		return fmt.Errorf("MaxRangeSize 必须在 0-%d 范围内，当前值: %d", MaxAllowedRangeSize, cfg.Server.MaxRangeSize)
	}

	// 验证 CacheTTL
	if cfg.Music.CacheTTLMinutes < 0 || cfg.Music.CacheTTLMinutes > MaxAllowedCacheTTL {
		return fmt.Errorf("CacheTTLMinutes 必须在 0-%d 范围内，当前值: %d", MaxAllowedCacheTTL, cfg.Music.CacheTTLMinutes)
	}

	// 验证音乐目录是否可读
	if _, err := os.Stat(cfg.Music.Directory); err != nil {
		return fmt.Errorf("音乐目录不可访问: %v", err)
	}

	return nil
}

// GetDefaultConfig 返回一个包含默认设置的配置实例。
func GetDefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	musicDir := filepath.Join(homeDir, "Music")
	// 如果默认的 Music 目录不存在，则使用当前工作目录下的 "music" 文件夹。
	if _, err := os.Stat(musicDir); os.IsNotExist(err) {
		musicDir, _ = filepath.Abs("./music")
	}

	return &Config{
		Server: ServerConfig{
			Host:         DefaultServerHost,
			Port:         DefaultServerPort,
			MaxRangeSize: DefaultMaxRangeSize,
		},
		Music: MusicConfig{
			Directory:        musicDir,
			SupportedFormats: []string{".mp3", ".flac", ".wav", ".m4a", ".ogg"},
			CacheTTLMinutes:  DefaultCacheTTLMinutes,
		},
	}
}
