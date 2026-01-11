package main

import (
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type cfg struct {
	port               string
	monolithURL        *url.URL
	moviesURL          *url.URL
	eventsURL          *url.URL
	gradualMigration   bool
	moviesMigrationPct int // 0..100
}

func main() {
	rand.Seed(time.Now().UnixNano())

	c := mustConfig()

	mux := http.NewServeMux()

	// movies route (strangler fig)
	mux.HandleFunc("/api/movies", func(w http.ResponseWriter, r *http.Request) {
		target := c.monolithURL
		if c.gradualMigration && shouldGoToMovies(c.moviesMigrationPct) {
			target = c.moviesURL
		}
		proxyTo(target, w, r)
	})

	// (на всякий случай) если тесты ходят на /api/movies/...
	mux.HandleFunc("/api/movies/", func(w http.ResponseWriter, r *http.Request) {
		target := c.monolithURL
		if c.gradualMigration && shouldGoToMovies(c.moviesMigrationPct) {
			target = c.moviesURL
		}
		proxyTo(target, w, r)
	})

	// events route (если вдруг gateway должен проксировать и это)
	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		proxyTo(c.eventsURL, w, r)
	})
	mux.HandleFunc("/api/events/", func(w http.ResponseWriter, r *http.Request) {
		proxyTo(c.eventsURL, w, r)
	})

	// everything else -> monolith
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		proxyTo(c.monolithURL, w, r)
	})

	addr := ":" + c.port
	log.Printf("proxy listening on %s (gradual=%v, movies=%d%%)", addr, c.gradualMigration, c.moviesMigrationPct)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func shouldGoToMovies(pct int) bool {
	if pct <= 0 {
		return false
	}
	if pct >= 100 {
		return true
	}
	return rand.Intn(100) < pct
}

func proxyTo(target *url.URL, w http.ResponseWriter, r *http.Request) {
	// собираю URL: target + original path + query
	outURL := *target
	outURL.Path = singleJoiningSlash(target.Path, r.URL.Path)
	outURL.RawQuery = r.URL.RawQuery

	req, err := http.NewRequest(r.Method, outURL.String(), r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// копирую заголовки (кроме host)
	for k, vv := range r.Header {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	req.Host = target.Host

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// копирую статус и заголовки
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	_, _ = io.Copy(w, resp.Body)
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	default:
		return a + b
	}
}

func mustConfig() cfg {
	port := getenv("PORT", "8000")

	monolithURL := mustURL(getenv("MONOLITH_URL", "http://monolith:8080"))
	moviesURL := mustURL(getenv("MOVIES_SERVICE_URL", "http://movies-service:8081"))
	eventsURL := mustURL(getenv("EVENTS_SERVICE_URL", "http://events-service:8082"))

	gradual := strings.ToLower(getenv("GRADUAL_MIGRATION", "false")) == "true"
	pctStr := getenv("MOVIES_MIGRATION_PERCENT", "0")
	pct, err := strconv.Atoi(pctStr)
	if err != nil {
		pct = 0
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	return cfg{
		port:               port,
		monolithURL:        monolithURL,
		moviesURL:          moviesURL,
		eventsURL:          eventsURL,
		gradualMigration:   gradual,
		moviesMigrationPct: pct,
	}
}

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func getenv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}
