package importtask

import (
	"html"
	"regexp"
	"strings"
)

type ParsedChapter struct {
	Title   string
	Content string
}

var tagPattern = regexp.MustCompile(`(?s)<[^>]+>`)
var headingPattern = regexp.MustCompile(`(?i)^(第\s*[0-9一二三四五六七八九十百千]+\s*章[^\r\n]*|chapter\s+[0-9]+[^\r\n]*)$`)

func ParseForumChapters(body []byte) []ParsedChapter {
	plain := html.UnescapeString(tagPattern.ReplaceAllString(string(body), "\n"))
	return parseTextChapters(plain, "")
}

// ParseTXTChapters parses a UTF-8 text import. A heading starts a chapter;
// when no heading is present the complete document becomes one chapter.
func ParseTXTChapters(body []byte, fallbackTitle string) []ParsedChapter {
	text := strings.TrimPrefix(string(body), "\ufeff")
	if strings.TrimSpace(fallbackTitle) == "" {
		fallbackTitle = "未命名章节"
	}
	return parseTextChapters(text, fallbackTitle)
}

func parseTextChapters(text, fallbackTitle string) []ParsedChapter {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
	chapters := []ParsedChapter{}
	current := -1
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" && current < 0 {
			continue
		}
		if line == "" {
			if chapters[current].Content != "" {
				chapters[current].Content += "\n"
			}
			continue
		}
		if headingPattern.MatchString(line) {
			chapters = append(chapters, ParsedChapter{Title: line})
			current = len(chapters) - 1
			continue
		}
		if current >= 0 {
			if chapters[current].Content != "" {
				chapters[current].Content += "\n"
			}
			chapters[current].Content += line
		}
	}
	for i := range chapters {
		chapters[i].Content = strings.TrimSpace(chapters[i].Content)
	}
	if current < 0 && strings.TrimSpace(text) != "" && strings.TrimSpace(fallbackTitle) != "" {
		title := strings.TrimSpace(fallbackTitle)
		chapters = append(chapters, ParsedChapter{Title: title, Content: strings.TrimSpace(strings.Join(lines, "\n"))})
	}
	return chapters
}
