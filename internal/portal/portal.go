package portal

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/MiloSharp/Stoa-Inspect/internal/eventio"
	"github.com/MiloSharp/Stoa-Inspect/internal/observer"
)

//go:embed index.html
var indexHTML []byte

type Server struct {
	source         string
	toolVersion    string
	includePrivate bool
	mu             sync.RWMutex
	lastGood       *observer.View
	lastError      string
}

type State struct {
	View  *observer.View `json:"view,omitempty"`
	Stale bool           `json:"stale"`
	Error string         `json:"error,omitempty"`
}

func New(source, toolVersion string, includePrivate bool) *Server {
	return &Server{source: source, toolVersion: toolVersion, includePrivate: includePrivate}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/api/state", s.state)
	mux.HandleFunc("/api/stream", s.stream)
	return securityHeaders(mux)
}

func (s *Server) Listen(address string) error {
	server := &http.Server{
		Addr:              address,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return server.ListenAndServe()
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

func (s *Server) state(w http.ResponseWriter, _ *http.Request) {
	state := s.load()
	w.Header().Set("Content-Type", "application/json")
	if state.View == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(state)
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case now := <-ticker.C:
			fmt.Fprintf(w, "event: update\ndata: %d\n\n", now.UnixMilli())
			flusher.Flush()
		}
	}
}

func (s *Server) load() State {
	view, err := readView(s.source, s.toolVersion, s.includePrivate)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.lastGood = &view
		s.lastError = ""
		return State{View: s.lastGood}
	}
	s.lastError = err.Error()
	return State{View: s.lastGood, Stale: s.lastGood != nil, Error: s.lastError}
}

func readView(source, toolVersion string, includePrivate bool) (observer.View, error) {
	reader, err := openSource(source)
	if err != nil {
		return observer.View{}, err
	}
	defer reader.Close()
	events, err := eventio.Read(reader)
	if err != nil {
		return observer.View{}, fmt.Errorf("read event stream: %w", err)
	}
	return observer.Build(events, source, toolVersion, includePrivate), nil
}

func openSource(source string) (io.ReadCloser, error) {
	parsed, err := url.Parse(source)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		if !isLoopbackHost(parsed.Hostname()) {
			return nil, fmt.Errorf("remote event URLs are not allowed: use a local Stoa endpoint")
		}
		client := &http.Client{Timeout: 5 * time.Second}
		response, err := client.Get(source)
		if err != nil {
			return nil, fmt.Errorf("request local Stoa state: %w", err)
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return nil, fmt.Errorf("local Stoa state returned %s", response.Status)
		}
		return response.Body, nil
	}
	file, err := os.Open(source)
	if err != nil {
		return nil, fmt.Errorf("open event stream: %w", err)
	}
	return file, nil
}

func ValidateSource(source string) error {
	parsed, err := url.Parse(source)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		if !isLoopbackHost(parsed.Hostname()) {
			return fmt.Errorf("remote event URLs are not allowed: use a local Stoa endpoint")
		}
		return nil
	}
	if strings.Contains(source, "://") {
		return fmt.Errorf("unsupported source URL")
	}
	_, err = os.Stat(source)
	return err
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func IsLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	return isLoopbackHost(host)
}

func isLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
}

func WriteStartup(w io.Writer, address, source string, includePrivate bool) {
	fmt.Fprintf(w, "STOA WATCH\nPortal: http://%s\nSource: %s\nPrivate events: %t\nPress Ctrl+C to stop.\n", address, source, includePrivate)
}
