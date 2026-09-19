package config

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// RouteConfig は動的ルーティングのルール定義
type RouteConfig struct {
	Prefix      string `json:"prefix"`                 // マッチさせるパスプレフィックス (例: "/mcp")
	Target      string `json:"target"`                 // 転送先ベースURL (例: "http://mcp-gateway:8000")
	Scope       string `json:"scope,omitempty"`        // 必要な API キースコープ (例: "mcp:*")
	StripPrefix bool   `json:"strip_prefix,omitempty"` // true の場合、転送時に Prefix を除去する
}

// Config は Tollgate アプリケーション全体の環境設定
type Config struct {
	Port             string
	DynamoDBEndpoint string
	AWSRegion        string
	TableName        string
	OpenAPIPath      string
	DocsPath         string
	KeyCacheTTL      time.Duration
	Routes           []*RouteConfig
	ForwardTargetURL string
	HashSecret       string
}

// Load は環境変数および設定ファイルから Config を構築する
func Load() (*Config, error) {
	port := cmp.Or(os.Getenv("PORT"), "8000")
	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	region := cmp.Or(os.Getenv("AWS_REGION"), "ap-northeast-1")
	tableName := cmp.Or(os.Getenv("TABLE_NAME"), "TollgateAPIKeys")
	hashSecret := cmp.Or(os.Getenv("HASH_SECRET"), "default-insecure-secret")

	// OpenAPI & Docs パス正規化 (空文字でなければ先頭スラッシュ補完)
	openapiPath := normalizePath(os.Getenv("OPENAPI_PATH"))
	docsPath := normalizePath(os.Getenv("DOCS_PATH"))

	// キーキャッシュ TTL (秒単位, 未指定時は 10 秒, 0 で無効化)
	keyCacheTTL := 10 * time.Second
	if ttlStr := os.Getenv("KEY_CACHE_TTL"); ttlStr != "" {
		if sec, err := strconv.Atoi(ttlStr); err == nil {
			if sec <= 0 {
				keyCacheTTL = 0
			} else {
				keyCacheTTL = time.Duration(sec) * time.Second
			}
		}
	}

	// ルート定義の読み込み
	routes, defaultTarget, err := loadRoutes()
	if err != nil {
		return nil, fmt.Errorf("failed to load proxy routes: %w", err)
	}

	return &Config{
		Port:             port,
		DynamoDBEndpoint: endpoint,
		AWSRegion:        region,
		TableName:        tableName,
		OpenAPIPath:      openapiPath,
		DocsPath:         docsPath,
		KeyCacheTTL:      keyCacheTTL,
		Routes:           routes,
		ForwardTargetURL: defaultTarget,
		HashSecret:       hashSecret,
	}, nil
}

func normalizePath(p string) string {
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

func loadRoutes() ([]*RouteConfig, string, error) {
	var routes []*RouteConfig

	// 1. 設定ファイルからの読み込み (ROUTES_CONFIG_FILE)
	if configFile := os.Getenv("ROUTES_CONFIG_FILE"); configFile != "" {
		if data, err := os.ReadFile(configFile); err == nil {
			var parsed []*RouteConfig
			if err := json.Unmarshal(data, &parsed); err == nil {
				routes = append(routes, parsed...)
			} else {
				log.Printf("[tollgate] Warning: failed to parse ROUTES_CONFIG_FILE: %v", err)
			}
		} else {
			log.Printf("[tollgate] Warning: failed to read ROUTES_CONFIG_FILE: %v", err)
		}
	}

	// 2. PROXY_ROUTES JSON 環境変数の読み込み
	if jsonEnv := os.Getenv("PROXY_ROUTES"); jsonEnv != "" {
		var parsed []*RouteConfig
		if err := json.Unmarshal([]byte(jsonEnv), &parsed); err == nil {
			routes = append(routes, parsed...)
		} else {
			log.Printf("[tollgate] Warning: failed to parse PROXY_ROUTES JSON: %v", err)
		}
	}

	defaultTarget := os.Getenv("FORWARD_TARGET_URL")

	return routes, defaultTarget, nil
}
