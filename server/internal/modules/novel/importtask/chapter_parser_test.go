package importtask

import "testing"

func TestParseForumChapters(t *testing.T) {
	got := ParseForumChapters([]byte(`<h2>第一章 初见</h2><p>正文一</p><h2>Chapter 2</h2><p>正文二 &amp; 更多</p>`))
	if len(got) != 2 || got[0].Title != "第一章 初见" || got[0].Content != "正文一" || got[1].Content != "正文二 & 更多" {
		t.Fatalf("chapters=%+v", got)
	}
}

func TestParseTXTChaptersSupportsBOMAndParagraphs(t *testing.T) {
	body := []byte("\ufeff第 001 章 初见\r\n第一段\r\n\r\n第二段\r\nChapter 2\n结尾")
	got := ParseTXTChapters(body, "整本文档")
	if len(got) != 2 {
		t.Fatalf("chapters=%+v", got)
	}
	if got[0].Title != "第 001 章 初见" || got[0].Content != "第一段\n\n第二段" {
		t.Fatalf("first chapter=%+v", got[0])
	}
	if got[1].Title != "Chapter 2" || got[1].Content != "结尾" {
		t.Fatalf("second chapter=%+v", got[1])
	}
}

func TestParseTXTChaptersFallsBackToSingleChapter(t *testing.T) {
	got := ParseTXTChapters([]byte("正文第一行\n正文第二行"), "导入文件")
	if len(got) != 1 || got[0].Title != "导入文件" || got[0].Content != "正文第一行\n正文第二行" {
		t.Fatalf("chapters=%+v", got)
	}
}
