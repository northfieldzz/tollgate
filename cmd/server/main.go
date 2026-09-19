package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/northfieldzz/tollgate/internal/config"
	deliveryHttp "github.com/northfieldzz/tollgate/internal/delivery/http"
	"github.com/northfieldzz/tollgate/internal/domain/repository"
	"github.com/northfieldzz/tollgate/internal/infrastructure/cache"
	infraDynamo "github.com/northfieldzz/tollgate/internal/infrastructure/dynamodb"
	"github.com/northfieldzz/tollgate/internal/infrastructure/ratelimit"
	"github.com/northfieldzz/tollgate/internal/usecase"
)

func main() {
	// 1. 環境設定の読み込み
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[tollgate] Failed to load configuration: %v", err)
	}

	log.Printf("[tollgate] Initializing with Region=%s, Endpoint=%s, Table=%s, CacheTTL=%v",
		cfg.AWSRegion, cfg.DynamoDBEndpoint, cfg.TableName, cfg.KeyCacheTTL)

	// 2. AWS SDK 初期化 (ローカル開発時のみダミークレデンシャルを適用)
	var optFns []func(*awsConfig.LoadOptions) error
	optFns = append(optFns, awsConfig.WithRegion(cfg.AWSRegion))

	if cfg.DynamoDBEndpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, reg string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           cfg.DynamoDBEndpoint,
				SigningRegion: cfg.AWSRegion,
			}, nil
		})
		optFns = append(optFns,
			awsConfig.WithEndpointResolverWithOptions(customResolver),
			awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("dummy", "dummy", "")),
		)
	}

	awsCfg, err := awsConfig.LoadDefaultConfig(context.Background(), optFns...)
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	// 3. リポジトリ・キャッシュ・ユースケース初期化
	var repo repository.KeyRepository = infraDynamo.NewDynamoDBRepository(dynamoClient, cfg.TableName)
	if cfg.KeyCacheTTL > 0 {
		repo = cache.NewCachedKeyRepository(repo, cfg.KeyCacheTTL)
	}

	limiter := ratelimit.NewSlidingWindowLimiter(time.Minute)
	defer limiter.Stop()

	keyUsecase := usecase.NewKeyUsecase(repo, cfg.HashSecret)
	verifyUsecase := usecase.NewVerifyUsecase(repo, limiter, cfg.HashSecret)

	// 4. 動的ルート定義 & リバースプロキシ構築
	var proxyHandler http.Handler
	if len(cfg.Routes) > 0 || cfg.ForwardTargetURL != "" {
		proxy, err := deliveryHttp.NewMultiTargetProxy(cfg.Routes, cfg.ForwardTargetURL, verifyUsecase)
		if err != nil {
			log.Fatalf("failed to initialize proxy: %v", err)
		}
		proxyHandler = proxy
	}

	// 5. ルーター・HTTP サーバー構築
	router := deliveryHttp.NewRouter(cfg, keyUsecase, verifyUsecase, repo, proxyHandler)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second, // Slowloris 対策
		IdleTimeout:       120 * time.Second,
		// SSE (Server-Sent Events) および WebSocket の常時接続を維持するため Read/WriteTimeout は無制限 (0)
	}

	// 6. サーバー起動
	go func() {
		if proxyHandler != nil {
			log.Printf("[tollgate] Reverse Proxy mode active with %d route(s):", len(cfg.Routes))
			for _, r := range cfg.Routes {
				log.Printf("[tollgate]   -> Prefix: %-8s => Target: %-30s (Scope: %-10s, StripPrefix: %v)", r.Prefix, r.Target, r.Scope, r.StripPrefix)
			}
			if cfg.ForwardTargetURL != "" {
				log.Printf("[tollgate]   -> Default fallback => %s", cfg.ForwardTargetURL)
			}
		} else {
			log.Println("[tollgate] Running in standalone API mode (No proxy routes configured)")
		}

		var infoMsgs []string
		if cfg.OpenAPIPath != "" {
			infoMsgs = append(infoMsgs, fmt.Sprintf("OpenAPI at http://localhost:%s%s", cfg.Port, cfg.OpenAPIPath))
		}
		if cfg.DocsPath != "" {
			infoMsgs = append(infoMsgs, fmt.Sprintf("Docs at http://localhost:%s%s", cfg.Port, cfg.DocsPath))
		}

		infoStr := ""
		if len(infoMsgs) > 0 {
			infoStr = " (" + strings.Join(infoMsgs, ", ") + ")"
		}
		log.Printf("[tollgate] Server listening on :%s%s", cfg.Port, infoStr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v", err)
		}
	}()

	// 7. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[tollgate] Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[tollgate] Forced shutdown: %v", err)
	}
	log.Println("[tollgate] Server stopped.")
}
