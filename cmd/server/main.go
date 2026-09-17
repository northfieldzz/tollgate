package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	deliveryHttp "github.com/northfieldzz/tollgate/internal/delivery/http"
	infraDynamo "github.com/northfieldzz/tollgate/internal/infrastructure/dynamodb"
	"github.com/northfieldzz/tollgate/internal/infrastructure/ratelimit"
	"github.com/northfieldzz/tollgate/internal/usecase"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "ap-northeast-1"
	}
	tableName := os.Getenv("TABLE_NAME")
	if tableName == "" {
		tableName = "ITCP_APIKeys"
	}

	log.Printf("[tollgate] Initializing with Region=%s, Endpoint=%s, Table=%s", region, endpoint, tableName)

	// 1. AWS SDK 初期化
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, reg string, options ...interface{}) (aws.Endpoint, error) {
		if endpoint != "" {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           endpoint,
				SigningRegion: region,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("dummy", "dummy", "")),
	)
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)

	// 2. リポジトリ・ユースケース初期化
	repo := infraDynamo.NewDynamoDBRepository(dynamoClient, tableName)
	limiter := ratelimit.NewSlidingWindowLimiter(time.Minute)

	keyUsecase := usecase.NewKeyUsecase(repo)
	verifyUsecase := usecase.NewVerifyUsecase(repo, limiter)

	// 3. 動的ルート定義の読み込み & リバースプロキシ構築
	routes, defaultTarget := deliveryHttp.LoadRoutesFromEnvOrConfig()
	var proxyHandler http.Handler
	if len(routes) > 0 || defaultTarget != "" {
		proxy, err := deliveryHttp.NewMultiTargetProxy(routes, defaultTarget, verifyUsecase)
		if err != nil {
			log.Fatalf("failed to initialize proxy: %v", err)
		}
		proxyHandler = proxy
	}

	// 4. ルーター・HTTP サーバー構築
	router := deliveryHttp.NewRouter(keyUsecase, verifyUsecase, repo, proxyHandler)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second, // Slowloris 対策
		IdleTimeout:       120 * time.Second,
		// SSE (Server-Sent Events) および WebSocket の常時接続を維持するため Read/WriteTimeout は無制限 (0)
	}

	// 5. Graceful Shutdown
	go func() {
		if proxyHandler != nil {
			log.Printf("[tollgate] Reverse Proxy mode active with %d route(s):", len(routes))
			for _, r := range routes {
				log.Printf("[tollgate]   -> Prefix: %-8s => Target: %-30s (Scope: %-10s, StripPrefix: %v)", r.Prefix, r.Target, r.Scope, r.StripPrefix)
			}
			if defaultTarget != "" {
				log.Printf("[tollgate]   -> Default fallback => %s", defaultTarget)
			}
		} else {
			log.Println("[tollgate] Running in standalone API mode (No proxy routes configured)")
		}
		log.Printf("[tollgate] Server listening on :%s (OpenAPI at http://localhost:%s/openapi.json)", port, port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v", err)
		}
	}()

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
