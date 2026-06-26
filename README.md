# 无损音乐下载器（网易云 / QQ音乐）

图形界面的音乐下载工具，支持 **网易云音乐** 与 **QQ音乐** 两个音源，可按 **歌单ID** 或 **单曲ID** 下载（优先 FLAC 无损）到本地 `songs_dir` 目录。

最初由 [Python 版本](https://github.com/YongHaoWu/NeteaseCloudMusicFlac) 使用 Golang 重写。原版依赖的百度接口已下线，现已改为直接调用各平台官方接口（网易云 weapi 加密 / QQ musicu + vkey）。

## 功能

- 图形窗口（基于 lxn/walk，Windows 原生，无需 cgo）。
- 音源可选：网易云音乐 / QQ音乐。
- 类型可选：歌单 / 单曲。
- 支持填入 Cookie 以获取无损（FLAC）/ VIP 资源。
- 并发下载，文件名按真实音质自动取扩展名（`.flac` / `.mp3`）。

## 编译

依赖窗口控件样式，需要把应用程序清单编译进 exe：

    # 1. 安装资源工具(仅首次)
    go install github.com/akavel/rsrc@latest

    # 2. 由清单生成资源(已提供 rsrc.syso, 修改清单后才需重跑)
    rsrc -manifest main.exe.manifest -arch amd64 -o rsrc.syso

    # 3. 编译(-H windowsgui 让双击只显示窗口, 不弹控制台)
    go build -ldflags "-H windowsgui" -o main.exe .

> 注：国内拉取依赖建议 `go env -w GOPROXY=https://goproxy.cn,direct`。

## 使用

双击 `main.exe` 打开窗口：

1. 选择 **音源**（网易云 / QQ音乐）与 **类型**（歌单 / 单曲）。
2. 在 **ID** 框填入：
   - 网易云歌单/单曲 ID（纯数字，如 `145258012`）。
   - QQ 歌单 disstid（纯数字）；QQ 单曲可填 `songmid`（字母数字）或数字 songid。
3. 需要无损/VIP 资源时，在 **Cookie** 框粘贴登录后的 Cookie。
4. 点击 **开始下载**，进度显示在下方日志区。文件保存到程序目录的 `songs_dir`。

### 命令行模式（便于自动化/测试）

带参数运行即进入 CLI（需用未加 `-H windowsgui` 的构建才能在终端看到输出）：

    main.exe netease playlist 145258012
    main.exe netease song 28151024
    main.exe qq playlist <disstid>
    main.exe qq song 004Z8Ihr0JIu5s
    main.exe "https://music.163.com/#/playlist?id=145258012"   # 兼容旧用法

## 关于无损（FLAC）与 Cookie

各平台的 FLAC / Hi-Res 属于 VIP / 付费内容，**必须提供已登录账号的 Cookie** 才能获取：

- **网易云**：Cookie 需包含 `MUSIC_U`。未登录时可拿到免费/标准音质（320/128 MP3），部分歌曲也提供免费 FLAC。
- **QQ音乐**：Cookie 需包含 `qm_keyst` / `qqmusic_key`（及 `uin`）。**未登录时接口基本不返回任何下载地址**（返回 `retcode 104009`），因此 QQ 音源几乎必须填 Cookie。

提供 Cookie 的方式（按优先级，靠前者优先生效）：

1. GUI 的 Cookie 输入框直接粘贴（仅当次使用）。
2. 环境变量：网易云 `NETEASE_COOKIE`、QQ音乐 `QQ_COOKIE`。
3. 程序目录的 `cookie.txt`（推荐，可长期保存两个平台的 Cookie）。

### cookie.txt 格式

每行一条 Cookie，程序按音源自动匹配：**含 `MUSIC_U` 的行用于网易云，含 `qm_keyst` 的行用于 QQ音乐**。可同时存放两条，互不影响。`#` 开头为注释。可选地加标签前缀（`netease:` / `qq:` / `网易云:` / `qq音乐:`），会被自动去除：

    # 我的 Cookie
    netease: os=pc; MUSIC_U=xxxxxxxx; __csrf=yy
    qq: uin=12345; qm_keyst=Q_H_L_zzz; qqmusic_key=Q_H_L_zzz

> 标签前缀仅为可读性，并非必需；只要某行包含 `MUSIC_U` 或 `qm_keyst` 即可被正确识别。
>
> `cookie.txt` 含登录凭证，已加入 `.gitignore`，请勿提交到仓库或外传。

> 获取方式：浏览器登录 music.163.com / y.qq.com → F12 → Network → 任意请求 Request Headers → 复制 Cookie。

## 代码结构

- `crypto.go`：网易云 weapi 加密（AES-128-CBC 二次加密 + 无填充 RSA）。
- `netease.go`：网易云接口客户端（歌单 / 单曲 / 下载地址）+ 统一 `Song` 结构。
- `qqmusic.go`：QQ 音乐接口客户端（musicu 取详情 + vkey 换地址，flac→320→128 逐档回退）。
- `provider.go`：音源/类型统一调度 `fetch` + 并发下载。
- `gui.go`：walk 图形界面。
- `main.go`：入口（无参启动 GUI，有参走 CLI）。
- `main.exe.manifest` / `rsrc.syso`：启用 Common-Controls 6.0 的应用程序清单及其编译资源。

#### 本程序仅供学习之用。
