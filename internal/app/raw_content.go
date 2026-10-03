package app

import "net/http"

// Repository bytes are untrusted same-origin content. Only inert raster image
// formats retain their MIME type; HTML, SVG and other active formats are text.
func serveRepositoryContent(w http.ResponseWriter, data []byte) {
	ct := http.DetectContentType(data)
	switch ct {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		ct = "text/plain; charset=utf-8"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	_, _ = w.Write(data)
}
