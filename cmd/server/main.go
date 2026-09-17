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
	deliveryHttp "github.com/northfieldzz/null_and_void_work_agent/apps/api_manager/internal/delivery/http"
	infraDynamo "github.com/northfieldzz/null_and_void_work_agent/apps/api_manager/internal/infrastructure/dynamodb"
	"github.com/northfieldzz/null_and_void_work_agent/apps/api_manager/internal/infrastructure/ratelimit"
	"github.com/northfieldzz/null_and_void_work_agent/apps/api_manager/internal/usecase"
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

	log.Printf("[api_manager] Initializing with Region=%s, Endpoint=%s, Table=%s", region, endpoint, tableName)

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

	// 3. ルーター・HTTP サーバー構築
	router := deliveryHttp.NewRouter(keyUsecase, verifyUsecase, repo)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 4. Graceful Shutdown
	go func() {
		log.Printf("[api_manager] Server listening on :%s (OpenAPI at http://localhost:%s/openapi.json)", port, port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[api_manager] Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[api_manager] Forced shutdown: %v", err)
	}
	log.Println("[api_manager] Server stopped.")
}
