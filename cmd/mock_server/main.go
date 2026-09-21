package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	port := "8000"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	http.HandleFunc("/", handler)

	log.Printf("[mock-server] Listening on :%s — logging all incoming requests", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("[mock-server] Fatal: %v", err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// ボディを読み込む（最大 64KB）
	body, _ := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	defer r.Body.Close()

	// ヘッダーを整形
	var headers strings.Builder
	for k, vs := range r.Header {
		headers.WriteString(fmt.Sprintf("    %s: %s\n", k, strings.Join(vs, ", ")))
	}

	bodyStr := ""
	if len(body) > 0 {
		bodyStr = fmt.Sprintf("\n  Body:\n    %s", strings.TrimSpace(string(body)))
	}

	log.Printf("[mock-server] %s %s\n  Headers:\n%s%s",
		r.Method, r.URL.RequestURI(),
		headers.String(),
		bodyStr,
	)

	// レスポンス
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"mock":true,"method":"%s","path":"%s","elapsed":"%s"}`,
		r.Method, r.URL.RequestURI(), time.Since(start))
}
