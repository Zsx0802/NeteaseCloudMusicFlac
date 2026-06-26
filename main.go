package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// loadCookie 按音源读取 Cookie.
// 优先级: 对应环境变量 > 当前目录 cookie.txt 中匹配该音源的行。
// cookie.txt 可同时存放两个平台的 Cookie(每行一条):
//   - 网易云: 含 MUSIC_U 的那一行
//   - QQ音乐: 含 qm_keyst 的那一行
// 可选地为行加标签前缀(netease: / qq: / 网易云: / qq音乐:), 会被自动去除。
func loadCookie(source Source) string {
	switch source {
	case SourceQQ:
		if c := strings.TrimSpace(os.Getenv("QQ_COOKIE")); c != "" {
			return c
		}
	case SourceNetease:
		if c := strings.TrimSpace(os.Getenv("NETEASE_COOKIE")); c != "" {
			return c
		}
	}
	data, err := os.ReadFile("cookie.txt")
	if err != nil {
		return ""
	}
	return pickCookie(string(data), source)
}

// pickCookie 从 cookie.txt 内容中挑出匹配音源的 Cookie 行.
func pickCookie(content string, source Source) string {
	token := "MUSIC_U"
	if source == SourceQQ {
		token = "qm_keyst"
	}
	var firstNonEmpty string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = stripCookieLabel(line)
		if firstNonEmpty == "" {
			firstNonEmpty = line
		}
		if strings.Contains(line, token) {
			return line
		}
	}
	// 兼容: 文件里只有一条未带标识 token 的 Cookie 时, 直接返回该行.
	if firstNonEmpty != "" && !strings.Contains(content, "MUSIC_U") && !strings.Contains(content, "qm_keyst") {
		return firstNonEmpty
	}
	return ""
}

// stripCookieLabel 去除可选的行首标签前缀(如 "netease:" / "qq:").
func stripCookieLabel(line string) string {
	for _, label := range []string{"netease:", "wangyiyun:", "网易云:", "网易:", "qqmusic:", "qq音乐:", "qq:"} {
		if len(line) >= len(label) && strings.EqualFold(line[:len(label)], label) {
			return strings.TrimSpace(line[len(label):])
		}
	}
	return line
}

// downloadDir 返回(并按需创建)下载目录 songs_dir.
func downloadDir() (string, error) {
	cwd, _ := os.Getwd()
	dir := filepath.Join(cwd, "songs_dir")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", err
	}
	return dir, nil
}

func main() {
	// 无参数 -> 启动图形界面; 有参数 -> 命令行模式(便于自动化/测试).
	if len(os.Args) <= 1 {
		runGUI()
		return
	}
	runCLI(os.Args[1:])
}

// runCLI 命令行模式.
// 用法: main.exe <netease|qq> <playlist|song> <id>
// 兼容: main.exe <网易云歌单链接或歌单ID>  (默认网易云歌单)
func runCLI(args []string) {
	var source Source
	var kind Kind
	var id string

	switch {
	case len(args) >= 3 && (args[0] == "netease" || args[0] == "qq"):
		source = Source(args[0])
		kind = Kind(args[1])
		id = args[2]
	default:
		// 向后兼容: 直接传网易云歌单链接/ID.
		source = SourceNetease
		kind = KindPlaylist
		id = parseNeteaseID(args[0])
	}

	dir, err := downloadDir()
	if err != nil {
		fmt.Println("创建目录失败:", err)
		return
	}
	cookie := loadCookie(source)
	logf := func(s string) { fmt.Println(s) }

	logf(fmt.Sprintf("音源=%s 类型=%s ID=%s", source, kind, id))
	title, songs, err := fetch(source, kind, id, cookie, logf)
	if err != nil {
		logf("获取失败: " + err.Error())
		return
	}
	logf(fmt.Sprintf("《%s》共 %d 首, 开始下载...", title, len(songs)))
	ok, skip := downloadSongs(songs, dir, logf)
	logf(fmt.Sprintf("完成: 成功 %d 首, 跳过 %d 首。目录: %s", ok, skip, dir))
}

// parseNeteaseID 从网易云歌单链接/纯数字中解析 ID.
func parseNeteaseID(arg string) string {
	arg = strings.TrimSpace(arg)
	if i := strings.Index(arg, "id="); i >= 0 {
		rest := arg[i+3:]
		end := strings.IndexAny(rest, "&?#/")
		if end >= 0 {
			rest = rest[:end]
		}
		return rest
	}
	return arg
}
