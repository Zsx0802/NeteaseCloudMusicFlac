package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// QQ 音乐 musicu 统一接口入口.
const qqMusicU = "https://u.y.qq.com/cgi-bin/musicu.fcg"

// qqFile 描述某首歌各音质是否可用(字节数>0 表示该音质存在).
type qqFile struct {
	MediaMid  string `json:"media_mid"`
	SizeFlac  int64  `json:"size_flac"`
	SizeHires int64  `json:"size_hires"`
	Size320   int64  `json:"size_320mp3"`
	Size128   int64  `json:"size_128mp3"`
	SizeApe   int64  `json:"size_ape"`
}

// qqTrack QQ 音乐歌曲基础信息.
type qqTrack struct {
	Mid    string `json:"mid"`
	Name   string `json:"name"`
	Title  string `json:"title"`
	Singer []struct {
		Name string `json:"name"`
	} `json:"singer"`
	File qqFile `json:"file"`
}

// toSong 转换为统一 Song(下载地址稍后填充).
func (t qqTrack) toSong() *Song {
	name := t.Name
	if name == "" {
		name = t.Title
	}
	singers := make([]string, 0, len(t.Singer))
	for _, s := range t.Singer {
		if s.Name != "" {
			singers = append(singers, s.Name)
		}
	}
	return &Song{
		Name:   name,
		Artist: strings.Join(singers, ","),
		QQMid:  t.Mid,
		qqFile: t.File,
	}
}

// qqMusicuGet 调用 musicu 接口, data 为请求体 JSON.
func qqMusicuGet(data string, cookie string) ([]byte, error) {
	u := qqMusicU + "?format=json&inCharset=utf8&outCharset=utf-8&data=" + url.QueryEscape(data)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "+
		"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://y.qq.com/")
	req.Header.Set("Origin", "https://y.qq.com")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码 %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// qqUinFromCookie 从 Cookie 中尽量解析 uin(用于登录态请求), 默认 "0".
// 兼容 QQ 登录(uin=o0123...) 与微信登录(wxuin=...) 两种 Cookie 形态.
func qqUinFromCookie(cookie string) string {
	// QQ 号登录: uin=o0123456789 / uin=123456789.
	if m := regexp.MustCompile(`(?:^|[;\s])uin=o?0*([1-9]\d+)`).FindStringSubmatch(cookie); m != nil {
		return m[1]
	}
	// 微信登录: 无 uin, 改用 wxuin.
	if m := regexp.MustCompile(`wxuin=0*([1-9]\d+)`).FindStringSubmatch(cookie); m != nil {
		return m[1]
	}
	return "0"
}

// qqCookieDiagnose 检查 Cookie 是否具备下载所需的登录字段, 返回提示(无问题则为空).
func qqCookieDiagnose(cookie string) string {
	if cookie == "" {
		return "未提供 Cookie。QQ音乐即使下载标准音质也几乎必须登录, 无损(FLAC)更需绿钻会员 Cookie。"
	}
	hasKey := strings.Contains(cookie, "qm_keyst") || strings.Contains(cookie, "qqmusic_key")
	hasUin := qqUinFromCookie(cookie) != "0"
	switch {
	case !hasKey && !hasUin:
		return "Cookie 缺少 qm_keyst/qqmusic_key 与 uin, 像是未登录或复制不完整。请重新登录 y.qq.com 后完整复制整段 Cookie。"
	case !hasKey:
		return "Cookie 缺少 qm_keyst/qqmusic_key(下载鉴权字段), 可能复制不完整或已过期。"
	case !hasUin:
		return "Cookie 中未找到 uin/wxuin, 可能复制不完整。微信登录账号请确认包含 wxuin。"
	}
	return ""
}

// fetchQQPlaylist 获取 QQ 音乐歌单(disstid)全部歌曲并填充下载地址.
func fetchQQPlaylist(disstid, cookie string, logf func(string)) (string, []*Song, error) {
	reqData := fmt.Sprintf(`{"comm":{"ct":24,"cv":0},"req_0":{"module":"music.srfDissInfo.aiDissInfo","method":"uniform_get_Dissinfo","param":{"disstid":%s,"enc_host_uin":"","tag":1,"userinfo":1,"song_begin":0,"song_num":1000}}}`, disstid)
	body, err := qqMusicuGet(reqData, cookie)
	if err != nil {
		return "", nil, err
	}
	var r struct {
		Req0 struct {
			Data struct {
				Dirinfo struct {
					Title string `json:"title"`
				} `json:"dirinfo"`
				Songlist []qqTrack `json:"songlist"`
			} `json:"data"`
		} `json:"req_0"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", nil, fmt.Errorf("解析 QQ 歌单失败: %v", err)
	}
	tracks := r.Req0.Data.Songlist
	if len(tracks) == 0 {
		return "", nil, fmt.Errorf("歌单为空或 ID 无效(disstid=%s)", disstid)
	}
	songs := make([]*Song, 0, len(tracks))
	for _, t := range tracks {
		songs = append(songs, t.toSong())
	}
	fillQQURLs(songs, cookie, logf)
	return r.Req0.Data.Dirinfo.Title, songs, nil
}

// fetchQQSong 获取 QQ 音乐单曲. id 可为数字 songid 或字母数字 songmid.
func fetchQQSong(id, cookie string, logf func(string)) ([]*Song, error) {
	id = strings.TrimSpace(id)
	var param string
	if regexp.MustCompile(`^\d+$`).MatchString(id) {
		param = fmt.Sprintf(`{"song_type":0,"song_id":%s,"song_mid":""}`, id)
	} else {
		param = fmt.Sprintf(`{"song_type":0,"song_id":0,"song_mid":"%s"}`, id)
	}
	reqData := fmt.Sprintf(`{"comm":{"ct":24,"cv":0},"req_0":{"module":"music.pf_song_detail_svr","method":"get_song_detail_yqq","param":%s}}`, param)
	body, err := qqMusicuGet(reqData, cookie)
	if err != nil {
		return nil, err
	}
	var r struct {
		Req0 struct {
			Data struct {
				TrackInfo qqTrack `json:"track_info"`
			} `json:"data"`
		} `json:"req_0"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("解析 QQ 单曲失败: %v", err)
	}
	t := r.Req0.Data.TrackInfo
	if t.Mid == "" {
		return nil, fmt.Errorf("未找到 QQ 单曲: %s", id)
	}
	songs := []*Song{t.toSong()}
	fillQQURLs(songs, cookie, logf)
	return songs, nil
}

// qqQuality 描述一种音质对应的文件名前缀/扩展名/码率.
type qqQuality struct {
	prefix string
	ext    string
	br     int // bps, 仅用于展示
	avail  func(qqFile) bool
}

// 按优先级从高到低: 无损 > 320 > 128.
var qqQualities = []qqQuality{
	{"F000", "flac", 999000, func(f qqFile) bool { return f.SizeFlac > 0 }},
	{"M800", "mp3", 320000, func(f qqFile) bool { return f.Size320 > 0 }},
	{"M500", "mp3", 128000, func(f qqFile) bool { return f.Size128 > 0 }},
}

// fillQQURLs 为歌曲批量申请 vkey 并组装下载地址.
// 逐档音质尝试: 先试高音质, 拿不到地址的歌再降档重试.
func fillQQURLs(songs []*Song, cookie string, logf func(string)) {
	if logf == nil {
		logf = func(string) {}
	}
	if tip := qqCookieDiagnose(cookie); tip != "" {
		logf("⚠ QQ音乐: " + tip)
	}
	uin := qqUinFromCookie(cookie)
	guid := fmt.Sprintf("%010d", rand.Int63n(9000000000)+1000000000)

	for _, q := range qqQualities {
		// 收集仍缺地址、且该音质可用的歌曲.
		pending := make([]*Song, 0, len(songs))
		for _, s := range songs {
			if s.URL == "" && s.QQMid != "" && q.avail(s.qqFile) {
				pending = append(pending, s)
			}
		}
		if len(pending) == 0 {
			continue
		}
		for _, chunk := range chunkSongs(pending, 100) {
			requestQQVkey(chunk, q, guid, uin, cookie, logf)
		}
	}

	// 统计仍无地址的歌曲, 给出可执行的原因提示.
	noURL := 0
	for _, s := range songs {
		if s.URL == "" {
			noURL++
		}
	}
	if noURL > 0 {
		logf(fmt.Sprintf("⚠ QQ音乐: %d/%d 首未取得下载地址。"+
			"常见原因: ① Cookie 已过期(请重新登录 y.qq.com 复制); "+
			"② 复制的 Cookie 不完整(需含 qm_keyst 与 uin/wxuin); "+
			"③ 该歌曲为数字专辑/独家, 会员也需单独购买。",
			noURL, len(songs)))
	}
}

// requestQQVkey 对一批歌曲按指定音质申请 vkey 并写回 URL.
func requestQQVkey(songs []*Song, q qqQuality, guid, uin, cookie string, logf func(string)) {
	mids := make([]string, len(songs))
	types := make([]int, len(songs))
	filenames := make([]string, len(songs))
	for i, s := range songs {
		mids[i] = s.QQMid
		types[i] = 0
		mediaMid := s.qqFile.MediaMid
		if mediaMid == "" {
			mediaMid = s.QQMid
		}
		filenames[i] = q.prefix + mediaMid + "." + q.ext
	}
	midsJSON, _ := json.Marshal(mids)
	typesJSON, _ := json.Marshal(types)
	filesJSON, _ := json.Marshal(filenames)

	reqData := fmt.Sprintf(`{"comm":{"ct":24,"cv":0,"uin":"%s"},"req_0":{"module":"vkey.GetVkeyServer","method":"CgiGetVkey","param":{"guid":"%s","songmid":%s,"songtype":%s,"uin":"%s","loginflag":1,"platform":"20","filename":%s}}}`,
		uin, guid, string(midsJSON), string(typesJSON), uin, string(filesJSON))

	body, err := qqMusicuGet(reqData, cookie)
	if err != nil {
		logf("QQ音乐: 获取下载地址出错: " + err.Error())
		return
	}
	var r struct {
		Code int `json:"code"`
		Req0 struct {
			Code int `json:"code"`
			Data struct {
				Sip        []string `json:"sip"`
				Midurlinfo []struct {
					Songmid  string `json:"songmid"`
					Purl     string `json:"purl"`
					Filename string `json:"filename"`
				} `json:"midurlinfo"`
			} `json:"data"`
		} `json:"req_0"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		logf("QQ音乐: 解析下载地址出错: " + err.Error())
		return
	}
	// req_0.code 非 0 即鉴权/接口异常, 例如 104009 多为未登录/Cookie 失效.
	if r.Req0.Code != 0 {
		hint := ""
		if r.Req0.Code == 104009 {
			hint = "(通常表示未登录或 Cookie 已失效, 请重新登录 y.qq.com 复制完整 Cookie)"
		}
		logf(fmt.Sprintf("QQ音乐: vkey 接口返回 code=%d %s [%s档]", r.Req0.Code, hint, strings.ToUpper(q.ext)))
	}
	sip := ""
	if len(r.Req0.Data.Sip) > 0 {
		sip = r.Req0.Data.Sip[0]
	}
	// midurlinfo 与请求顺序一致.
	for i, info := range r.Req0.Data.Midurlinfo {
		if i >= len(songs) || info.Purl == "" || sip == "" {
			continue
		}
		songs[i].URL = sip + info.Purl
		songs[i].Type = q.ext
		songs[i].Br = q.br
	}
}

// chunkSongs 把 Song 列表按 size 分块.
func chunkSongs(songs []*Song, size int) [][]*Song {
	var chunks [][]*Song
	for i := 0; i < len(songs); i += size {
		end := i + size
		if end > len(songs) {
			end = len(songs)
		}
		chunks = append(chunks, songs[i:end])
	}
	return chunks
}
