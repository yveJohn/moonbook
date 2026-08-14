package importtask

import "testing"

func TestParseForumChapters(t *testing.T) {
	got := ParseForumChapters([]byte(`<h2>第一章 初见</h2><p>正文一</p><h2>Chapter 2</h2><p>正文二 &amp; 更多</p>`))
	if len(got) != 2 || got[0].Title != "第一章 初见" || got[0].Content != "正文一" || got[1].Content != "正文二 & 更多" {
		t.Fatalf("chapters=%+v", got)
	}
}
