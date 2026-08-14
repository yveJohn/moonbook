package candidate

import (
	"net/url"
	"testing"
)

func TestParseDiscoveredDiscuzLinks(t *testing.T) {
	page, _ := url.Parse("https://forum.example.test/forum")
	base, _ := url.Parse("https://forum.example.test")
	items := parseDiscovered([]byte(`<a href="thread-123-1-1.html">第一帖</a><a href="viewthread.php?tid=456">第二帖</a><a href="https://other.example/thread-999">外站</a>`), page, base)
	if len(items) != 2 {
		t.Fatalf("items=%+v", items)
	}
}
