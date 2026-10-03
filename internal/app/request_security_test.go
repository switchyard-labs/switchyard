package app

import (
	"net/http/httptest"
	"testing"
)

func TestOriginAndURLSchemes(t *testing.T) {
	t.Setenv("SWITCHYARD_PUBLIC_URL", "https://switchyard.example")
	for _, origin := range []string{"https://evil.example", "null", "http://switchyard.example", "https://switchyard.example.evil"} {
		r := httptest.NewRequest("POST", "https://switchyard.example/api/drafts", nil)
		r.Header.Set("Origin", origin)
		if sameOrigin(r) {
			t.Fatal(origin)
		}
	}
	r := httptest.NewRequest("POST", "https://switchyard.example/api/drafts", nil)
	r.Header.Set("Origin", "https://switchyard.example")
	if !sameOrigin(r) || !secureRequest(r) {
		t.Fatal("same-origin https rejected")
	}
	for _, link := range []string{"javascript:alert(1)", "data:text/html,test", "java\tscript:alert(1)", "//evil.example", "https://u:p@example.com"} {
		if safeWebURL(link) {
			t.Fatal(link)
		}
	}
	if !safeWebURL("https://example.com/path") {
		t.Fatal("https rejected")
	}
}
func TestLoginThrottle(t *testing.T) {
	a := &App{}
	for i := 0; i < 10; i++ {
		if !a.allowLogin("127.0.0.1:123") {
			t.Fatal(i)
		}
	}
	if a.allowLogin("127.0.0.1:456") {
		t.Fatal("port bypass")
	}
	if !a.allowLogin("127.0.0.2:123") {
		t.Fatal("other client")
	}
}
