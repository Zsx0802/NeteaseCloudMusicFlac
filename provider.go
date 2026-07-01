package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// 并发下载数.
const concurrency = 10

// Source 音源.
type Source string

const (
	SourceNetease Source = "netease" // 网易云音乐
	SourceQQ      Source = "qq"      // QQ 音乐
)

// Kind 资源类型.
type Kind string

const (
	KindPlaylist Kind = "playlist" // 歌单
	KindSong     Kind = "song"     // 单曲
	KindAlbum    Kind = "album"    // 专辑
	KindSinger   Kind = "singer"   // 歌手热门
)

// fetch 统一调度: 按音源/类型获取歌曲列表(已填好下载地址).
// 返回标题(歌单名或单曲名)与歌曲列表. logf 用于输出诊断信息(可为 nil).
func fetch(source Source, kind Kind, id, cookie string, logf func(string)) (string, []*Song, error) {
	if logf == nil {
		logf = func(string) {}
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", nil, fmt.Errorf("请输入 ID")
	}
	switch source {
	case SourceNetease:
		if kind == KindPlaylist {
			return fetchNeteasePlaylist(id, cookie)
		}
		songs, err := fetchNeteaseSong(id, cookie)
		return "单曲", songs, err
	case SourceQQ:
		switch kind {
		case KindPlaylist:
			return fetchQQPlaylist(id, cookie, logf)
		case KindAlbum:
			return fetchQQAlbum(id, cookie, logf)
		case KindSinger:
			return fetchQQSinger(id, 30, cookie, logf)
		}
		songs, err := fetchQQSong(id, cookie, logf)
		return "单曲", songs, err
	default:
		return "", nil, fmt.Errorf("未知音源: %s", source)
	}
}

// downloadSongs 并发下载歌曲到 dir, 通过 logf 输出进度. 返回成功/跳过数量.
func downloadSongs(songs []*Song, dir string, logf func(string)) (okCount, skipCount int) {
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, s := range songs {
		if s == nil {
			continue
		}
		if s.URL == "" {
			logf(fmt.Sprintf("× 跳过 [%s - %s]: 无可用下载地址(可能需VIP/版权受限)。", s.Name, s.Artist))
			mu.Lock()
			skipCount++
			mu.Unlock()
			continue
		}
		ext := s.Type
		if ext == "" {
			ext = "mp3"
		}
		filename := filepath.Join(dir, sanitizeFilename(s.Name+"-"+s.Artist)+"."+ext)

		// 已存在且大小与接口一致 -> 跳过重下(断点续传).
		if s.Size > 0 {
			if fi, err := os.Stat(filename); err == nil && fi.Size() == s.Size {
				logf(fmt.Sprintf("↻ 已存在跳过 [%s - %s] %.2fMB", s.Name, s.Artist, float64(fi.Size())/(1024*1024)))
				mu.Lock()
				skipCount++
				mu.Unlock()
				continue
			}
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(s *Song, filename string) {
			defer func() {
				<-sem
				wg.Done()
			}()
			written, err := downloadFile(s.URL, filename, s.Size)
			if err != nil {
				logf(fmt.Sprintf("下载失败 [%s - %s]: %v", s.Name, s.Artist, err))
				return
			}
			mu.Lock()
			okCount++
			mu.Unlock()
			logf(fmt.Sprintf("✓ [%s - %s] %s %dkbps %.2fMB",
				s.Name, s.Artist, strings.ToUpper(s.Type), s.Br/1000, float64(written)/(1024*1024)))
		}(s, filename)
	}
	wg.Wait()
	return
}

// sanitizeFilename 去除文件名中的非法字符.
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"\\", "_", "/", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	name = strings.TrimSpace(replacer.Replace(name))
	if name == "" {
		name = "unknown"
	}
	return name
}

// downloadFile 下载 url 到 dst, 返回写入字节数.
func downloadFile(url, dst string, expected int64) (int64, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "+
		"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP \u72b6\u6001\u7801 %d", resp.StatusCode)
	}

	// \u5148\u5199\u5165 .part \u4e34\u65f6\u6587\u4ef6, \u5b8c\u6210\u5e76\u6821\u9a8c\u540e\u518d\u539f\u5b50\u91cd\u547d\u540d, \u907f\u514d\u4e2d\u65ad\u4ea7\u751f\u635f\u574f\u6587\u4ef6.
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}

	written, err := io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return 0, err
	}

	// \u5927\u5c0f\u6821\u9a8c: \u82e5\u63a5\u53e3\u7ed9\u4e86\u9884\u671f\u5927\u5c0f\u4e14\u4e0d\u4e00\u81f4, \u5224\u4e3a\u4e0b\u8f7d\u4e0d\u5b8c\u6574.
	if expected > 0 && written != expected {
		os.Remove(tmp)
		return 0, fmt.Errorf("\u4e0b\u8f7d\u4e0d\u5b8c\u6574: \u5b9e\u9645 %d \u5b57\u8282, \u9884\u671f %d \u5b57\u8282", written, expected)
	}

	// \u76ee\u6807\u5df2\u5b58\u5728\u65f6\u5148\u5220\u9664, \u4fdd\u8bc1 rename \u6210\u529f(Windows).
	os.Remove(dst)
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return 0, err
	}
	return written, nil
}
