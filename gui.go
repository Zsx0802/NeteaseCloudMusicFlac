package main

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// 蓝白主题配色.
var (
	colPrimary = walk.RGB(45, 127, 249)  // 主蓝 #2D7FF9
	colPriDark = walk.RGB(28, 100, 210)  // 深蓝
	colWindow  = walk.RGB(244, 248, 255) // 浅蓝白底
	colCard    = walk.RGB(255, 255, 255) // 卡片白
	colMuted   = walk.RGB(127, 140, 160) // 次要文字
	colSubLine = walk.RGB(208, 224, 255) // 顶栏副标题
)

// 字体.
var (
	fontUI    = Font{Family: "Microsoft YaHei UI", PointSize: 10}
	fontTitle = Font{Family: "Microsoft YaHei UI", PointSize: 17, Bold: true}
	fontSub   = Font{Family: "Microsoft YaHei UI", PointSize: 9}
	fontField = Font{Family: "Microsoft YaHei UI", PointSize: 10, Bold: true}
	fontBtn   = Font{Family: "Microsoft YaHei UI", PointSize: 10, Bold: true}
)

// runGUI 启动图形界面.
func runGUI() {
	var (
		mw          *walk.MainWindow
		sourceCB    *walk.ComboBox
		kindCB      *walk.ComboBox
		idEdit      *walk.LineEdit
		cookieEdit  *walk.LineEdit
		logEdit     *walk.TextEdit
		downloadBtn *walk.PushButton
	)

	appendLog := func(s string) {
		if mw == nil {
			return
		}
		mw.Synchronize(func() {
			logEdit.AppendText(s + "\r\n")
		})
	}

	sources := []string{"网易云音乐", "QQ音乐"}
	kinds := []string{"歌单", "单曲"}

	err := MainWindow{
		AssignTo:   &mw,
		Title:      "无损音乐下载器",
		Background: SolidColorBrush{Color: colWindow},
		Font:       fontUI,
		MinSize:    Size{Width: 600, Height: 580},
		Size:       Size{Width: 640, Height: 620},
		Layout:     VBox{MarginsZero: true, SpacingZero: true},
		Children: []Widget{
			// ===== 顶部蓝色标题栏 =====
			Composite{
				Background: SolidColorBrush{Color: colPrimary},
				Layout: VBox{Margins: Margins{Left: 24, Top: 18, Right: 24, Bottom: 18},
					Spacing: 4, Alignment: AlignHNearVNear},
				Children: []Widget{
					Label{Text: "无损音乐下载器", TextColor: colCard, Font: fontTitle},
					Label{Text: "网易云音乐 · QQ音乐    |    歌单 / 单曲    |    FLAC 无损优先",
						TextColor: colSubLine, Font: fontSub},
					// 横向弹簧: 让蓝色顶栏自适应铺满左右整宽.
					HSpacer{},
				},
			},

			// ===== 表单区域 =====
			Composite{
				Background: SolidColorBrush{Color: colWindow},
				Layout:     VBox{Margins: Margins{Left: 24, Top: 18, Right: 24, Bottom: 8}, Spacing: 14},
				Children: []Widget{
					// 设置卡片(白底)
					Composite{
						Background: SolidColorBrush{Color: colCard},
						Layout: Grid{
							Columns: 4,
							Margins: Margins{Left: 18, Top: 18, Right: 18, Bottom: 18},
							Spacing: 14,
						},
						Children: []Widget{
							Label{Text: "音源", TextColor: colPriDark, Font: fontField},
							ComboBox{AssignTo: &sourceCB, Model: sources, CurrentIndex: 0,
								MinSize: Size{Width: 130}, Font: fontUI},
							Label{Text: "类型", TextColor: colPriDark, Font: fontField},
							ComboBox{AssignTo: &kindCB, Model: kinds, CurrentIndex: 0,
								MinSize: Size{Width: 130}, Font: fontUI},

							Label{Text: "ID", TextColor: colPriDark, Font: fontField},
							LineEdit{AssignTo: &idEdit, ColumnSpan: 3, Font: fontUI,
								CueBanner: "歌单ID / 单曲ID,如 145258012;QQ单曲可填 songmid"},

							Label{Text: "Cookie", TextColor: colPriDark, Font: fontField},
							LineEdit{AssignTo: &cookieEdit, ColumnSpan: 3, PasswordMode: true, Font: fontUI,
								CueBanner: "可选,下载无损/VIP 需填,含 MUSIC_U 或 qm_keyst"},
						},
					},

					// 操作按钮
					Composite{
						Layout: HBox{Spacing: 10, MarginsZero: true},
						Children: []Widget{
							PushButton{AssignTo: &downloadBtn, Text: "开始下载", Font: fontBtn,
								MinSize: Size{Width: 120, Height: 38},
								OnClicked: func() {
									startDownload(sourceCB, kindCB, idEdit, cookieEdit, downloadBtn, appendLog)
								}},
							PushButton{Text: "清空日志", Font: fontUI, MinSize: Size{Height: 38},
								OnClicked: func() { logEdit.SetText("") }},
							PushButton{Text: "打开下载目录", Font: fontUI, MinSize: Size{Height: 38},
								OnClicked: func() {
									if dir, err := downloadDir(); err == nil {
										_ = exec.Command("explorer", dir).Start()
									}
								}},
							HSpacer{},
						},
					},

					Label{Text: "文件保存到程序目录的 songs_dir。未登录时多为 320/128 MP3;QQ音乐基本必须填 Cookie。",
						TextColor: colMuted, Font: fontSub},
				},
			},

			// ===== 日志区域 =====
			Composite{
				Background: SolidColorBrush{Color: colWindow},
				Layout:     VBox{Margins: Margins{Left: 24, Top: 0, Right: 24, Bottom: 18}, Spacing: 6},
				Children: []Widget{
					Label{Text: "下载日志", TextColor: colPriDark, Font: fontField},
					TextEdit{AssignTo: &logEdit, ReadOnly: true, VScroll: true,
						Font: Font{Family: "Consolas", PointSize: 9}, MinSize: Size{Height: 250}},
				},
			},
		},
	}.Create()

	if err != nil {
		fmt.Println("创建窗口失败:", err)
		return
	}

	mw.Run()
}

// startDownload 读取界面输入并在后台执行下载.
func startDownload(sourceCB, kindCB *walk.ComboBox, idEdit, cookieEdit *walk.LineEdit, btn *walk.PushButton, logf func(string)) {
	id := idEdit.Text()
	if id == "" {
		logf("请输入 歌单ID / 单曲ID。")
		return
	}

	var source Source
	if sourceCB.CurrentIndex() == 1 {
		source = SourceQQ
	} else {
		source = SourceNetease
	}
	var kind Kind
	if kindCB.CurrentIndex() == 1 {
		kind = KindSong
	} else {
		kind = KindPlaylist
	}
	cookie := cookieEdit.Text()
	if cookie == "" {
		cookie = loadCookie(source) // 退回到 cookie.txt(按音源匹配 MUSIC_U / qm_keyst)
	}

	btn.SetEnabled(false)
	btn.SetText("下载中...")
	go func() {
		defer btn.Synchronize(func() {
			btn.SetEnabled(true)
			btn.SetText("开始下载")
		})

		dir, err := downloadDir()
		if err != nil {
			logf("创建目录失败: " + err.Error())
			return
		}
		if cookie == "" {
			logf("⚠ 未提供 Cookie,无损(FLAC)可能获取不到,将下载可用的标准音质。")
		}
		logf(fmt.Sprintf("[%s] 正在解析 %s: %s ...", time.Now().Format("15:04:05"), kind, id))

		title, songs, err := fetch(source, kind, id, cookie, logf)
		if err != nil {
			logf("获取失败: " + err.Error())
			return
		}
		logf(fmt.Sprintf("《%s》共 %d 首,开始下载...", title, len(songs)))

		ok, skip := downloadSongs(songs, dir, logf)
		logf(fmt.Sprintf("全部完成: 成功 %d 首, 跳过 %d 首。目录: %s", ok, skip, dir))
	}()
}
