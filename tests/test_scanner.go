package main

import (
	"fmt"
	"log"
	"zero-music/services"
)

func main() {
	fmt.Println("=== 测试音乐文件扫描功能 ===")
	fmt.Println()

	// 指定要扫描的音乐目录。
	musicDir := "./music"

	fmt.Printf("扫描目录: %s\n", musicDir)
	fmt.Println()

	// 创建一个新的音乐扫描器实例。
	scanner := services.NewMusicScanner(musicDir, []string{".mp3", ".flac", ".wav"}, 5)

	// 执行扫描操作。
	songs, err := scanner.Scan()
	if err != nil {
		log.Fatalf("扫描失败: %v", err)
	}

	// 显示扫描结果。
	fmt.Println("扫描完成!")
	fmt.Printf("共找到 %d 首歌曲\n", scanner.GetSongCount())
	fmt.Println()

	if len(songs) > 0 {
		fmt.Println("歌曲列表:")
		for i, song := range songs {
			fmt.Printf("%d. %s\n", i+1, song.Title)
			fmt.Printf("   文件名: %s\n", song.FileName)
			fmt.Printf("   大小: %.2f MB\n", float64(song.FileSize)/(1024*1024))
			fmt.Printf("   路径: %s\n", song.FilePath)
			fmt.Println()
		}
	} else {
		fmt.Println("在指定目录中未找到支持的音乐文件。")
		fmt.Printf("请确认音乐文件（如 .mp3, .flac, .wav）已放置在目录: %s\n", musicDir)
	}
}
