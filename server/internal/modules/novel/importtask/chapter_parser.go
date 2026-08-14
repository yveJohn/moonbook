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
	lines := strings.Split(strings.ReplaceAll(plain, "\r\n", "\n"), "\n")
	chapters := []ParsedChapter{}
	current := -1
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
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
	return chapters
}
