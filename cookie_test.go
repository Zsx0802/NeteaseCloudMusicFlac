package main

import "testing"

func TestPickCookie(t *testing.T) {
	content := "" +
		"# 我的 Cookie\n" +
		"netease: os=pc; MUSIC_U=abc123; __csrf=xx\n" +
		"qq: uin=12345; qm_keyst=Q_H_L_xyz; qqmusic_key=Q_H_L_xyz\n"

	if got := pickCookie(content, SourceNetease); got != "os=pc; MUSIC_U=abc123; __csrf=xx" {
		t.Errorf("netease 取错: %q", got)
	}
	if got := pickCookie(content, SourceQQ); got != "uin=12345; qm_keyst=Q_H_L_xyz; qqmusic_key=Q_H_L_xyz" {
		t.Errorf("qq 取错: %q", got)
	}
}

func TestPickCookieNoLabel(t *testing.T) {
	// 无标签前缀, 仅靠 token 区分.
	content := "MUSIC_U=aaa; foo=1\nuin=9; qm_keyst=bbb\n"
	if got := pickCookie(content, SourceNetease); got != "MUSIC_U=aaa; foo=1" {
		t.Errorf("netease: %q", got)
	}
	if got := pickCookie(content, SourceQQ); got != "uin=9; qm_keyst=bbb" {
		t.Errorf("qq: %q", got)
	}
}

func TestPickCookieSingleUnlabeled(t *testing.T) {
	// 文件里只有一条不含 token 的内容时, 兼容性返回该行.
	content := "some=cookie; value=1\n"
	if got := pickCookie(content, SourceNetease); got != "some=cookie; value=1" {
		t.Errorf("fallback: %q", got)
	}
}

func TestPickCookieMissing(t *testing.T) {
	// 只有网易云 Cookie 时, QQ 应返回空.
	content := "MUSIC_U=aaa; foo=1\n"
	if got := pickCookie(content, SourceQQ); got != "" {
		t.Errorf("qq 应为空, 实际 %q", got)
	}
}
