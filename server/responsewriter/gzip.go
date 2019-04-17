package responsewriter

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"
)

type wrapWriter struct {
	gzipResponseWriter

	code int
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (g gzipResponseWriter) Write(b []byte) (int, error) {
	return g.Writer.Write(b)
}

func Gzip(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			handler.ServeHTTP(w, r)
			return
		}
		gz := gzip.NewWriter(w)
		defer gz.Close()

		gzw := &wrapWriter{gzipResponseWriter{Writer: gz, ResponseWriter: w}, http.StatusOK}
		gzw.Header().Set("Content-Encoding", "gzip")
		handler.ServeHTTP(gzw, r)
	})
}

// Must implement Hijacker to properly chain with handlers expecting a hijacker handler to be passed
func (g *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := g.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("Upstream ResponseWriter of type %v does not implement http.Hijacker", reflect.TypeOf(g.ResponseWriter))
}
