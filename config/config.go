package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 定义了应用程序的所有配置项。
type Config struct {
	Server ServerConfig `json:"server"`
	Music  MusicConfig  `json:"music"`
}

// ServerConfig 定义了服务器相关的配置。
type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
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

var globalConfig *Config

// Load 从指定的路径加载配置文件。
// 如果 configPath 为空，则返回默认配置。
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
		cfg.Music.CacheTTLMinutes = 5
	}

	// 将音乐目录的相对路径转换为绝对路径。
	if !filepath.IsAbs(cfg.Music.Directory) {
		absPath, err := filepath.Abs(cfg.Music.Directory)
		if err == nil {
			cfg.Music.Directory = absPath
		}
	}

	globalConfig = &cfg
	return &cfg, nil
}

// Get 返回全局唯一的配置实例。
func Get() *Config {
	if globalConfig == nil {
		globalConfig = GetDefaultConfig()
	}
	return globalConfig
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
			Host: "0.0.0.0",
			Port: 8080,
		},
		Music: MusicConfig{
			Directory:        musicDir,
			SupportedFormats: []string{".mp3", ".flac", ".wav", ".m4a", ".ogg"},
			CacheTTLMinutes:  5,
		},
	}
}