package importtask

import (
	"bytes"
	"errors"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

func findNextPage(body []byte, current, origin *url.URL) (*url.URL, error) {
	document, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("parse forum pagination")
	}
	var href string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if href != "" {
			return
		}
		if node.Type == html.ElementNode && (node.Data == "a" || node.Data == "link") && isNextNode(node) {
			for _, attribute := range node.Attr {
				if strings.EqualFold(attribute.Key, "href") {
					href = strings.TrimSpace(attribute.Val)
					break
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	if href == "" {
		return nil, nil
	}
	reference, err := url.Parse(href)
	if err != nil {
		return nil, errors.New("invalid forum next-page url")
	}
	next := current.ResolveReference(reference)
	next.Fragment = ""
	if next.User != nil || (next.Scheme != "http" && next.Scheme != "https") || !sameOrigin(next, origin) {
		return nil, errors.New("forum next page leaves thread origin")
	}
	return next, nil
}

func isNextNode(node *html.Node) bool {
	for _, attribute := range node.Attr {
		switch strings.ToLower(attribute.Key) {
		case "rel":
			if tokenContains(attribute.Val, "next") {
				return true
			}
		case "class":
			if tokenContains(attribute.Val, "nxt") || tokenContains(attribute.Val, "next") {
				return true
			}
		case "aria-label":
			value := strings.TrimSpace(strings.ToLower(attribute.Val))
			if value == "next" || value == "下一页" {
				return true
			}
		}
	}
	text := strings.TrimSpace(strings.ToLower(nodeText(node)))
	return text == "next" || text == "next page" || text == "下一页" || text == "下页"
}

func tokenContains(value, expected string) bool {
	for _, token := range strings.Fields(strings.ToLower(value)) {
		if token == expected {
			return true
		}
	}
	return false
}

func nodeText(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}
