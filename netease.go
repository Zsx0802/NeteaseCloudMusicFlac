package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const weapiBase = "https://music.163.com/weapi/"

// Song 表示一首歌的元信息与下载信息.
type Song struct {
	ID     int64
	Name   string
	Artist string
	URL    string // 下载地址(可能为空)
	Type   string // 音质类型: flac / mp3 ...
	Br     int    // 码率(bps)
	Size   int64  // 文件大小(字节)

	// QQ 音乐专用字段.
	QQMid  string // QQ 歌曲 mid
	qqFile qqFile // QQ 各音质可用情况
}

// weapiPost 以 weapi 加密方式 POST 请求网易云接口并返回响应体.
func weapiPost(path string, payload map[string]interface{}, cookie string) ([]byte, error) {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	if _, ok := payload["csrf_token"]; !ok {
		payload["csrf_token"] = ""
	}
	form := weapiEncrypt(payload)

	req, err := http.NewRequest("POST", weapiBase+path+"?csrf_token=", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "+
		"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://music.163.com")
	req.Header.Set("Origin", "https://music.163.com")
	// os=pc 必带; 若提供了登录 Cookie(含 MUSIC_U)则可获取 VIP/无损资源.
	ck := "os=pc; appver=8.9.70; "
	if cookie != "" {
		ck += cookie
	}
	req.Header.Set("Cookie", ck)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码 %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// getPlaylistTrackIDs 获取歌单内全部歌曲 ID 及歌单名.
func getPlaylistTrackIDs(playlistID, cookie string) (string, []int64, error) {
	payload := map[string]interface{}{
		"id":      playlistID,
		"n":       100000,
		"s":       8,
		"offset":  0,
		"total":   true,
		"limit":   1000,
	}
	body, err := weapiPost("v6/playlist/detail", payload, cookie)
	if err != nil {
		return "", nil, err
	}

	var r struct {
		Code     int    `json:"code"`
		Msg      string `json:"msg"`
		Playlist struct {
			Name     string `json:"name"`
			TrackIds []struct {
				ID int64 `json:"id"`
			} `json:"trackIds"`
		} `json:"playlist"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", nil, fmt.Errorf("解析歌单失败: %v", err)
	}
	if r.Code != 200 {
		return "", nil, fmt.Errorf("接口返回错误码 %d: %s", r.Code, r.Msg)
	}
	ids := make([]int64, 0, len(r.Playlist.TrackIds))
	for _, t := range r.Playlist.TrackIds {
		ids = append(ids, t.ID)
	}
	return r.Playlist.Name, ids, nil
}

// getSongDetails 批量获取歌曲名与歌手名, 返回 id -> Song.
func getSongDetails(ids []int64, cookie string) map[int64]*Song {
	result := make(map[int64]*Song, len(ids))
	for _, chunk := range chunkIDs(ids, 500) {
		var c strings.Builder
		c.WriteByte('[')
		for i, id := range chunk {
			if i > 0 {
				c.WriteByte(',')
			}
			fmt.Fprintf(&c, `{"id":%d}`, id)
		}
		c.WriteByte(']')

		body, err := weapiPost("v3/song/detail", map[string]interface{}{"c": c.String()}, cookie)
		if err != nil {
			fmt.Println("获取歌曲详情出错:", err)
			continue
		}
		var r struct {
			Code  int `json:"code"`
			Songs []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
				Ar   []struct {
					Name string `json:"name"`
				} `json:"ar"`
			} `json:"songs"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			fmt.Println("解析歌曲详情出错:", err)
			continue
		}
		for _, s := range r.Songs {
			artists := make([]string, 0, len(s.Ar))
			for _, a := range s.Ar {
				if a.Name != "" {
					artists = append(artists, a.Name)
				}
			}
			result[s.ID] = &Song{
				ID:     s.ID,
				Name:   s.Name,
				Artist: strings.Join(artists, ","),
			}
		}
	}
	return result
}

// fillSongURLs 批量请求下载地址, 写入对应 Song.
// br=999000 表示请求最高音质(无损/Hi-Res), 实际能否拿到取决于歌曲与账号权限.
func fillSongURLs(ids []int64, songs map[int64]*Song, cookie string) {
	for _, chunk := range chunkIDs(ids, 500) {
		var b strings.Builder
		b.WriteByte('[')
		for i, id := range chunk {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "%d", id)
		}
		b.WriteByte(']')

		payload := map[string]interface{}{
			"ids": b.String(),
			"br":  999000,
		}
		body, err := weapiPost("song/enhance/player/url", payload, cookie)
		if err != nil {
			fmt.Println("获取下载地址出错:", err)
			continue
		}
		var r struct {
			Code int `json:"code"`
			Data []struct {
				ID   int64  `json:"id"`
				URL  string `json:"url"`
				Br   int    `json:"br"`
				Size int64  `json:"size"`
				Type string `json:"type"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			fmt.Println("解析下载地址出错:", err)
			continue
		}
		for _, d := range r.Data {
			s := songs[d.ID]
			if s == nil {
				s = &Song{ID: d.ID}
				songs[d.ID] = s
			}
			s.URL = d.URL
			s.Br = d.Br
			s.Size = d.Size
			s.Type = strings.ToLower(d.Type)
			if s.Type == "" {
				s.Type = "mp3"
			}
		}
	}
}

// fetchNeteasePlaylist 获取网易云歌单, 返回歌单名与有序歌曲列表(已填好下载地址).
func fetchNeteasePlaylist(playlistID, cookie string) (string, []*Song, error) {
	name, ids, err := getPlaylistTrackIDs(playlistID, cookie)
	if err != nil {
		return "", nil, err
	}
	songs := getSongDetails(ids, cookie)
	fillSongURLs(ids, songs, cookie)
	list := make([]*Song, 0, len(ids))
	for _, id := range ids {
		if s := songs[id]; s != nil {
			list = append(list, s)
		}
	}
	return name, list, nil
}

// fetchNeteaseSong 获取网易云单曲(已填好下载地址).
func fetchNeteaseSong(songID, cookie string) ([]*Song, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(songID), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("网易云单曲 ID 必须为数字: %s", songID)
	}
	ids := []int64{id}
	songs := getSongDetails(ids, cookie)
	fillSongURLs(ids, songs, cookie)
	if s := songs[id]; s != nil {
		return []*Song{s}, nil
	}
	return nil, fmt.Errorf("未获取到单曲信息: %s", songID)
}

// chunkIDs 把 id 列表按 size 分块.
func chunkIDs(ids []int64, size int) [][]int64 {
	var chunks [][]int64
	for i := 0; i < len(ids); i += size {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[i:end])
	}
	return chunks
}
