package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewMusicScanner 测试 NewMusicScanner 是否能正确创建一个扫描器实例。
func TestNewMusicScanner(t *testing.T) {
	scanner := NewMusicScanner("/test/dir", []string{".mp3"}, 5)
	if scanner == nil {
		t.Fatal("期望扫描器被成功创建")
	}
	if scanner.directory != "/test/dir" {
		t.Errorf("期望目录为 /test/dir, 得到 %s", scanner.directory)
	}
	if scanner.cacheTTL != 5*time.Minute {
		t.Errorf("期望缓存 TTL 为 5m, 得到 %v", scanner.cacheTTL)
	}
}

// TestMusicScanner_Scan 测试 Scan 方法是否能正确扫描并识别音乐文件。
func TestMusicScanner_Scan(t *testing.T) {
	// 创建一个临时目录用于测试。
	tmpDir := t.TempDir()

	// 创建一些假的 MP3 文件。
	testFile1 := filepath.Join(tmpDir, "test1.mp3")
	testFile2 := filepath.Join(tmpDir, "test2.mp3")
	if err := os.WriteFile(testFile1, []byte("fake mp3 content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile2, []byte("another fake mp3"), 0644); err != nil {
		t.Fatal(err)
	}

	// 创建一个非 MP3 文件，这个文件应该被忽略。
	txtFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtFile, []byte("text"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewMusicScanner(tmpDir, []string{".mp3"}, 5)
	songs, err := scanner.Scan()

	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	if len(songs) != 2 {
		t.Errorf("期望找到 2 首歌曲, 得到 %d", len(songs))
	}

	// 验证扫描到的歌曲信息是否基本完整。
	for _, song := range songs {
		if song.ID == "" {
			t.Error("歌曲 ID 为空")
		}
		if song.FilePath == "" {
			t.Error("歌曲 FilePath 为空")
		}
		if song.FileSize == 0 {
			t.Error("歌曲 FileSize 为 0")
		}
	}
}

// TestMusicScanner_ScanCache 测试扫描器的缓存机制是否正常工作。
func TestMusicScanner_ScanCache(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.mp3")
	if err := os.WriteFile(testFile, []byte("fake mp3"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewMusicScanner(tmpDir, []string{".mp3"}, 5)

	// 第一次扫描，应该会执行实际的扫描操作。
	songs1, err := scanner.Scan()
	if err != nil {
		t.Fatalf("第一次扫描失败: %v", err)
	}

	firstScanTime := scanner.lastScan

	// 立即进行第二次扫描，应该会命中缓存。
	songs2, err := scanner.Scan()
	if err != nil {
		t.Fatalf("第二次扫描失败: %v", err)
	}

	// 验证 lastScan 时间戳没有改变，证明缓存被使用。
	if scanner.lastScan != firstScanTime {
		t.Error("缓存未生效，lastScan 时间已改变")
	}

	// 验证两次扫描返回的歌曲数量相同。
	if len(songs1) != len(songs2) {
		t.Errorf("期望歌曲数量相同, 得到 %d 和 %d", len(songs1), len(songs2))
	}
}

// TestMusicScanner_Refresh 测试 Refresh 方法是否能强制刷新缓存。
func TestMusicScanner_Refresh(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.mp3")
	if err := os.WriteFile(testFile, []byte("fake mp3"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewMusicScanner(tmpDir, []string{".mp3"}, 5)

	// 第一次扫描。
	_, err := scanner.Scan()
	if err != nil {
		t.Fatalf("第一次扫描失败: %v", err)
	}

	firstScanTime := scanner.lastScan

	// 等待一小段时间以确保时间戳会不同。
	time.Sleep(10 * time.Millisecond)

	// 手动刷新缓存。
	err = scanner.Refresh()
	if err != nil {
		t.Fatalf("刷新失败: %v", err)
	}

	// 验证 lastScan 时间戳已被更新。
	if scanner.lastScan == firstScanTime {
		t.Error("Refresh 方法未能更新 lastScan 时间")
	}
}

// TestMusicScanner_ScanNonExistentDirectory 测试当扫描一个不存在的目录时是否返回错误。
func TestMusicScanner_ScanNonExistentDirectory(t *testing.T) {
	scanner := NewMusicScanner("/non/existent/directory", []string{".mp3"}, 5)

	_, err := scanner.Scan()
	if err == nil {
		t.Error("期望在扫描不存在的目录时返回错误")
	}
}

// TestMusicScanner_GetSongs 测试 GetSongs 方法是否能正确返回歌曲列表。
func TestMusicScanner_GetSongs(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.mp3")
	if err := os.WriteFile(testFile, []byte("fake mp3"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewMusicScanner(tmpDir, []string{".mp3"}, 5)

	// 在扫描前调用，应返回空列表。
	songs := scanner.GetSongs()
	if len(songs) != 0 {
		t.Errorf("期望在扫描前歌曲数量为 0, 得到 %d", len(songs))
	}

	// 在扫描后调用，应返回扫描到的歌曲。
	scanner.Scan()
	songs = scanner.GetSongs()
	if len(songs) != 1 {
		t.Errorf("期望在扫描后歌曲数量为 1, 得到 %d", len(songs))
	}
}

// TestMusicScanner_GetSongCount 测试 GetSongCount 方法是否能正确返回歌曲数量。
func TestMusicScanner_GetSongCount(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.mp3")
	if err := os.WriteFile(testFile, []byte("fake mp3"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewMusicScanner(tmpDir, []string{".mp3"}, 5)

	// 扫描前。
	count := scanner.GetSongCount()
	if count != 0 {
		t.Errorf("期望在扫描前数量为 0, 得到 %d", count)
	}

	// 扫描后。
	scanner.Scan()
	count = scanner.GetSongCount()
	if count != 1 {
		t.Errorf("期望在扫描后数量为 1, 得到 %d", count)
	}
}

// TestMusicScanner_ConcurrentAccess 测试在并发访问下扫描器是否线程安全。
func TestMusicScanner_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.mp3")
	if err := os.WriteFile(testFile, []byte("fake mp3"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewMusicScanner(tmpDir, []string{".mp3"}, 5)

	// 使用 channel 来等待所有 goroutine 完成。
	done := make(chan bool, 3)

	// Goroutine 1: 持续调用 Scan。
	go func() {
		for i := 0; i < 10; i++ {
			scanner.Scan()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 2: 持续调用 GetSongs。
	go func() {
		for i := 0; i < 10; i++ {
			scanner.GetSongs()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 3: 持续调用 GetSongCount。
	go func() {
		for i := 0; i < 10; i++ {
			scanner.GetSongCount()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// 等待所有测试 goroutine 执行完毕。
	for i := 0; i < 3; i++ {
		<-done
	}
}
