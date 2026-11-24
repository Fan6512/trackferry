// Package webui serves the management UI built from web/ into dist/.
package webui

import (
	"bytes"
	"compress/gzip"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// pack compresses the text files once, at the start.
func pack(root fs.FS) map[string][]byte {
	out := map[string][]byte{}
	fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch path.Ext(p) {
		case ".js", ".css", ".svg":
		default:
			return nil
		}
		raw, err := fs.ReadFile(root, p)
		if err != nil || len(raw) < 1024 {
			return nil
		}
		var b bytes.Buffer
		z, _ := gzip.NewWriterLevel(&b, gzip.BestCompression)
		z.Write(raw)
		z.Close()
		out[p] = b.Bytes()
		return nil
	})
	return out
}

// Handler serves the built UI with an index.html fallback for client-side routes.
func Handler() http.Handler {
	root, _ := fs.Sub(dist, "dist")
	files := http.FileServerFS(root)
	packed := pack(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p != "" && p != "." {
			if fi, err := fs.Stat(root, p); err == nil && !fi.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				if z, ok := packed[p]; ok {
					w.Header().Set("Vary", "Accept-Encoding")
					if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
						w.Header().Set("Content-Encoding", "gzip")
						w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(p)))
						w.Header().Set("Content-Length", strconv.Itoa(len(z)))
						if r.Method != http.MethodHead {
							w.Write(z)
						}
						return
					}
				}
				files.ServeHTTP(w, r)
				return
			}
			if path.Ext(p) != "" {
				http.NotFound(w, r)
				return
			}
		}
		index, err := fs.ReadFile(root, "index.html")
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("TrackFerry web UI is not built into this binary. Run `npm run build` in web/.\n"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}
