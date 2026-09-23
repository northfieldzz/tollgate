# Tollgate — Lightweight API Key Management & Rate Limiting Gateway

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Huma v2](https://img.shields.io/badge/Huma-v2.39+-8A2BE2.svg)](https://huma.rocks/)
[![DynamoDB](https://img.shields.io/badge/Storage-DynamoDB%20%7C%20PostgreSQL%20%7C%20SQLite-4053D6.svg)](https://aws.amazon.com/dynamodb/)
[![Redis / Valkey](https://img.shields.io/badge/RateLimit-Memory%20%7C%20Redis%20%7C%20Valkey-DC382D.svg)](https://valkey.io/)
[![Prometheus](https://img.shields.io/badge/Prometheus-Metrics-E6522C.svg?logo=prometheus)](https://prometheus.io/)
[![OpenAPI](https://img.shields.io/badge/OpenAPI-3.1-6BA539.svg?logo=openapiinitiative)](https://spec.openapis.org/oas/v3.1.0)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

**Tollgate** は、マルチテナント SaaS・マイクロサービス基盤向けの軽量な API キー管理 & レートリミッティング・リバースプロキシである。  
マルチテナントに対応した API キー発行・ライフサイクル管理、スライディングウィンドウ方式による RPM 流量制御、月間クォータ管理、および動的マルチターゲット・リバースプロキシを単一バイナリ / コンテナで完結させる。

[Portico (MCP Gateway)](https://github.com/northfieldzz/portico) や LLM Gateway など、AI エージェント基盤の手前に配置してテナントコンテキストの安全な注入に使うこともできる。

---

## 主な機能

- **マルチストレージ & ゼロ依存起動モード**:
  - **DynamoDB**: AWS 完全マネージド、GSI スパースインデックス対応。
  - **PostgreSQL**: リレーショナル DB での運用（pgx 経由、コネクションプール最適化）。
  - **SQLite**: CGO 不要ピュア Go 実装。外部コンテナなし・バイナリ 1 本で即座に起動可能（開発・PoC・シングルノード用途に最適）。
- **柔軟なレートリミットバックエンド**:
  - **In-Memory**: 超低遅延なスライディングウィンドウカウンター（SQLite モード時は自動固定）。
  - **Redis / Valkey**: 分散スケールアウト環境向けの共有スライディングウィンドウ。
  - **DynamoDB**: AWS 完全マネージドなアトミックカウンター（※ PostgreSQL との混在は非推奨）。
- **動的マルチターゲット・リバースプロキシ**:
  - パスプレフィックス（`/llm`, `/mcp`, `/ai` 等）に基づき、各バックエンドへ自動ルーティング。
  - ルーティング単位での Prefix Stripping、スコープ検証（`llm:*`, `mcp:*` 等）を自動実行。
  - プロキシ専用 HTTP トランスポートチューニングによる高並行・低遅延通信（コネクションプール最適化、TIME_WAIT 枯渇抑止）。
- **堅牢なマルチテナント分離 & コンテキスト注入**:
  - **テナントキー**: `tenant_id` と `service_id` を保持。キーの `tenant_id` と `service_id` を下流へ `X-Tenant-ID` / `X-Service-ID` として確実に注入。クライアント指定値とのコンフリクト時は `403 Forbidden` で即座に遮断（Fail-Fast）。
  - **サービスキー**: `service_id` を保持し、クライアントが指定した動的 `X-Tenant-ID` を透過フォワード（未指定時は `400 Bad Request`）。下流にサービス識別子（`X-Service-ID`）を注入。
- **高スループット・レートリミット & クォータ制御**:
  - **分間レートリミット (RPM)**: スライディングウィンドウカウンターによる超低レイテンシなリアルタイム流量制限。
  - **月間クォータ**: アトミックカウンターによる月間利用回数の確実な集計と上限超過検知。
- **ゼロダウンタイム・キーローテーション**:
  - `POST /v1/admin/keys/{key_id}/rotate` により、旧キーの失効猶予期間（Grace Period）を保ちながら新キーを発行。クライアント側の無停止キー切り替えを支援。
- **クラウドネイティブ・オブザーバビリティ**:
  - **Kubernetes 標準プローブ**: `/livez`（Liveness）、`/readyz`（Readiness / DB 接続確認）、`/healthz`（総合確認）。
  - **Prometheus メトリクス**: `/metrics` で各種リクエスト数・レイテンシを公開。
  - **OpenAPI 3.1 & ドキュメント UI**: Huma v2 による OpenAPI 仕様（`/openapi.json`）および Scalar UI（`/docs`）の内蔵。
- **セキュアな設計原則 (Fail-Fast)**:
  - 平文 API キーは一切保存せず、SHA-256 ダイジェストのみを永続化。平文キーは発行時・ローテーション時に 1 度だけ返却。
  - デフォルトのフォールバックシークレットをコード内にハードコードせず、未設定時は起動時・検証時に即座にエラーとする安全設計。

---

## バックエンド組み合わせ

| DB バックエンド (`DB_BACKEND`) | レートリミット (`RATE_LIMIT_BACKEND`) | キャッシュ層 | 推奨用途 |
|:---|:---|:---:|:---|
| `sqlite` | `memory` (固定) | なし (ダイレクト) | **ゼロ外部依存・ローカル開発・PoC・単一バイナリ起動** |
| `dynamodb` | `memory` (デフォルト) または `redis` / `dynamodb` | あり | **AWS ネイティブ・サーバーレス構成** |
| `postgres` | `redis` (推奨) または `memory` | あり | **汎用 RDBMS・分散スケールアウト構成** |

> [!NOTE]
> `DB_BACKEND=postgres` かつ `RATE_LIMIT_BACKEND=dynamodb` の組み合わせは技術的には動作しますが、クラウド依存が混在するため**非推奨**です。PostgreSQL 採用時は `redis`（または `memory`）をご利用ください。

---

## 類似 OSS との比較

> [!NOTE]
> 下記はプロジェクト公式情報に基づく概要比較です。各プロジェクトは活発に開発されているため、最新の詳細は各公式ドキュメントで確認してください。

|  | **Tollgate** | **Kong Gateway (OSS)** | **Tyk Gateway (OSS)** | **Unkey** |
|:---|:---:|:---:|:---:|:---:|
| **主要な外部依存** | DynamoDB のみ | PostgreSQL（Traditional モード）または DB-less | Redis / Valkey（必須） | MySQL 互換 DB |
| **エディション分割** | なし（OSS 単一） | OSS 版と Enterprise 版で機能差あり | OSS 版と Enterprise 版で機能差あり（Dashboard 等） | コアは AGPL-3.0 |
| **リバースプロキシ機能** | あり（単一バイナリで完結） | あり | あり | 主軸は API キー管理・認証（ゲートウェイ機能は付随） |
| **デプロイの手軽さ** | 単一 Go バイナリ + DynamoDB | プラグイン学習コスト・複数コンポーネント構成 | Redis 必須・エコシステム全体の構築が必要 | Docker 対応、SQL DB 別途必要 |
| **マルチテナント対応** | ネイティブ（テナント / サービスキー二段構成） | プラグイン設定で実現 | プラグイン設定で実現 | API キー単位での管理 |

---

## アーキテクチャ

```mermaid
flowchart TD
    Client["Client / SDK / AI Agent"]
    
    subgraph TollgateCluster ["Tollgate (API Gateway : 8000)"]
        PROXY["Multi-Target Reverse Proxy"]
        
        subgraph Engine ["Core Engine"]
            VERIFY["API Key Verifier<br/>(SHA-256 Hash Matching)"]
            LIMITER["Sliding Window Limiter<br/>(In-Memory RPM)"]
            CACHE["Key Metadata Cache<br/>(TTL Cache)"]
            HEADER_INJECT["Context Injector<br/>(X-Tenant-ID / X-Key-ID)"]
        end

        subgraph ManagementAPI ["Management API (Huma v2 - Protected by ADMIN_API_KEY)"]
            KEY_MGMT["Key Lifecycle API<br/>• POST /v1/admin/keys<br/>• POST /v1/admin/keys/{id}/rotate<br/>• POST /v1/admin/verify"]
            HEALTH["Health & Probe Handlers<br/>• /livez<br/>• /readyz<br/>• /healthz"]
            METRICS["Prometheus<br/>• /metrics"]
        end
    end

    subgraph Storage ["Storage Layer"]
        DDB[("Amazon DynamoDB<br/>• TollgateAPIKeys<br/>• GSI_TenantKeys")]
    end

    subgraph Downstream ["Downstream Services (任意のバックエンド群)"]
        SVC_A["Service A<br/>(例: Prefix /service-a 転送先)"]
        SVC_B["Service B<br/>(例: Prefix /service-b 転送先)"]
        SVC_N["Any Microservices...<br/>(PROXY_ROUTES 定義先)"]
    end

    %% Client flows
    Client -->|"API Request (Bearer tlge-live-...)"| PROXY
    Client -->|"Admin Request (Bearer ADMIN_API_KEY)"| KEY_MGMT

    %% Internal Tollgate flows
    PROXY --> VERIFY
    VERIFY --> CACHE
    CACHE -.->|"Cache Miss"| DDB
    VERIFY --> LIMITER
    LIMITER -.->|"Quota Increment"| DDB
    PROXY --> HEADER_INJECT

    %% Forwarding
    HEADER_INJECT -->|"Prefix A マッチ (X-Tenant-ID 注入 / StripPrefix)"| SVC_A
    HEADER_INJECT -->|"Prefix B マッチ (X-Tenant-ID 注入 / StripPrefix)"| SVC_B
    HEADER_INJECT -->|"動的ルーティング転送"| SVC_N

    KEY_MGMT --> DDB
    HEALTH -.->|"Ping"| DDB
```

---

## ディレクトリ構成

```text
tollgate/
├── cmd/
│   └── server/                     # アプリケーションエントリポイント (main.go)
├── internal/
│   ├── config/                     # 環境変数・ルーティング設定ローダー
│   ├── delivery/
│   │   └── http/                   # HTTP ハンドラー、ルーティング、プロキシ実装
│   │       ├── api.go              # Huma v2 ルーター & 共通ミドルウェア初期化 (ADMIN_API_KEY 認証)
│   │       ├── health_handler.go   # /livez, /readyz, /healthz ハンドラー
│   │       ├── key_handler.go      # /v1/admin/keys CRUD & ローテーション API
│   │       ├── metrics_handler.go  # Prometheus /metrics ハンドラー
│   │       ├── proxy_handler.go    # 動的マルチターゲット・リバースプロキシ
│   │       └── verify_handler.go   # /v1/admin/verify 内部検証 API
│   ├── domain/
│   │   ├── entity/                 # ドメインモデル (APIKey, CreateKeyInput 等)
│   │   └── repository/             # リポジトリインターフェース定義
│   ├── infrastructure/
│   │   ├── cache/                  # キーメタデータ TTL インメモリキャッシュ
│   │   ├── dynamodb/               # DynamoDB SDK クライアント実装 & スパースインデックス
│   │   ├── metrics/                # Prometheus メトリクス定義
│   │   └── ratelimit/              # スライディングウィンドウ・レートリミッター
│   └── usecase/                    # キー管理 & 検証ユースケースロジック
├── docs/                           # 詳細仕様ドキュメント
│   └── backend_integration.md      # 下流サービス連携 & ヘッダー解決規約
├── compose.yaml                    # ローカル開発・検証用 Docker Compose 定義
├── Dockerfile                      # マルチステージビルド Dockerfile
├── CONTRIBUTING.md                 # コントリビューションガイド
├── SECURITY.md                     # セキュリティポリシー
└── README.md                       # 本ドキュメント
```

---

## API エンドポイント一覧

### 1. 管理用 API キー操作 (`/v1/admin/keys`)
> [!IMPORTANT]
> `/v1/admin/*` 配下のエンドポイントはマスター管理者キー（`Authorization: Bearer <ADMIN_API_KEY>` または `X-Admin-Key: <ADMIN_API_KEY>`）による認証が必須です。

| メソッド | パス | 説明 |
|:---|:---|:---|
| `POST` | `/v1/admin/keys` | 新規 API キー発行（平文キーは本レスポンスのみ開示） |
| `GET` | `/v1/admin/keys` | 指定テナントの API キー一覧取得 (`?tenant_id=...`) |
| `GET` | `/v1/admin/keys/{key_id}` | API キー詳細メタデータ取得 |
| `PATCH` | `/v1/admin/keys/{key_id}` | API キー設定変更（名称、スコープ、レート上限等） |
| `POST` | `/v1/admin/keys/{key_id}/suspend` | API キーの一時停止（即座にリクエスト拒絶） |
| `POST` | `/v1/admin/keys/{key_id}/resume` | 一時停止中 API キーの再開 |
| `POST` | `/v1/admin/keys/{key_id}/rotate` | ゼロダウンタイム・キーローテーション（新キー発行 & 猶予期間設定） |
| `DELETE` | `/v1/admin/keys/{key_id}` | API キーの物理削除・即時失効 |

### 2. キー検証 API (`/v1/admin/verify`)
| メソッド | パス | 説明 |
|:---|:---|:---|
| `POST` | `/v1/admin/verify` | 内部サービス連携用キー検証 & レートリミット / クォータ判定（ADMIN_API_KEY 認証必須） |

### 3. リバースプロキシ (`/*`)
| メソッド | パス | 説明 |
|:---|:---|:---|
| `ANY` | `/*` | `PROXY_ROUTES` にマッチした下流サービスへ認証・ヘッダー付与して透過転送 |

### 4. 運用 & オブザーバビリティ
| メソッド | パス | 説明 |
|:---|:---|:---|
| `GET` | `/livez` | **Liveness プローブ** (プロセスの死活監視、即座に 200 返却) |
| `GET` | `/readyz` | **Readiness プローブ** (DynamoDB 疎通確認、受付準備完了判定) |
| `GET` | `/healthz` | **総合ヘルスチェック** (プロセス生存 + DynamoDB 疎通状態) |
| `GET` | `/metrics` | **Prometheus メトリクス** (リクエスト数、レイテンシ等) |
| `GET` | `OPENAPI_PATH` | OpenAPI 3.1 仕様 JSON (例: `/openapi.json`, 環境変数指定時のみ有効) |
| `GET` | `DOCS_PATH` | Scalar ドキュメント UI (例: `/docs`, 環境変数指定時のみ有効) |

---

## 環境変数設定

主要な環境変数（詳細は [`.env.example`](.env.example) を参照）：

| 変数名 | デフォルト値 | 必須 | 説明 |
|:---|:---|:---:|:---|
| `ADMIN_API_KEY` | *(空)* | 推奨 | 管理用 WebAPI (`/v1/admin/*`) を保護するマスターキー。未設定時は管理 API が 401 で遮断される (Fail-Fast) |
| `DB_BACKEND` | `dynamodb` | 任意 | DB バックエンド (`dynamodb`, `sqlite`, `postgres`) |
| `SQLITE_PATH` | `./tollgate.db` | 任意 | SQLite データベースファイルパス (`DB_BACKEND=sqlite` 時) |
| `POSTGRES_DSN` | *(空)* | 任意 | PostgreSQL 接続 DSN (`DB_BACKEND=postgres` 時。`DATABASE_URL` も利用可) |
| `RATE_LIMIT_BACKEND`| `memory` | 任意 | レートリミットバックエンド (`memory`, `redis`, `dynamodb`) |
| `REDIS_ADDR` | `redis:6379` | 任意 | Redis / Valkey ホスト・ポート (`RATE_LIMIT_BACKEND=redis` 時) |
| `REDIS_PASSWORD` | *(空)* | 任意 | Redis / Valkey 認証パスワード |
| `REDIS_DB` | `0` | 任意 | Redis / Valkey DB 番号 |
| `PORT` | `8000` | 任意 | HTTP サーバーのリッスンポート |
| `AWS_REGION` | `ap-northeast-1` | 任意 | DynamoDB 接続リージョン |
| `TABLE_NAME` | `TollgateAPIKeys` | 任意 | API キー管理用 DynamoDB テーブル名 |
| `DYNAMODB_ENDPOINT` | *(AWS デフォルト)* | 任意 | DynamoDB Local 等のエンドポイント URL |
| `KEY_CACHE_TTL` | `10` | 任意 | キー検証メタデータのインメモリキャッシュ保持秒数 (`0` で無効化) |
| `PROXY_ROUTES` | *(空)* | 任意 | 動的ルーティング定義 (JSON 配列文字列) |
| `ROUTES_CONFIG_FILE`| *(空)* | 任意 | ルーティング設定ファイルのローカルパス |
| `FORWARD_TARGET_URL`| *(空)* | 任意 | ルート未マッチ時のフォールバック先 URL (未指定時は 404) |
| `OPENAPI_PATH` | *(空)* | 任意 | OpenAPI 3.1 スキーマ公開パス (未指定時は非公開) |
| `DOCS_PATH` | *(空)* | 任意 | Scalar ドキュメント UI 公開パス (未指定時は非公開) |

---

## クイックスタート

### 前提条件
- Docker / nerdctl & Docker Compose
- （ローカル実行時）Go 1.24+

### 1. Docker Compose での一括起動 (推奨)

DynamoDB Local、初期テーブル作成、DynamoDB Admin、および Tollgate をワンコマンドで起動する。

```bash
# 1. 環境変数の準備
cp .env.example .env

# 2. コンテナ起動 (DynamoDB + Admin + Tollgate)
nerdctl compose --profile database up -d --build

# 3. ログ確認
nerdctl compose --profile database logs -f tollgate

# 4. ヘルスチェック確認
curl -i http://localhost:8002/livez
curl -i http://localhost:8002/readyz
```

- **Tollgate ゲートウェイ**: `http://localhost:8002`
- **Scalar ドキュメント**: `http://localhost:8002/docs`
- **DynamoDB Admin UI**: `http://localhost:8003`

### 2. ローカル環境での起動 (Go)

```bash
# 依存解決
go mod download

# サーバー起動
go run cmd/server/main.go
```

---

## API 利用例

### ① テナントキーの発行 (`POST /v1/admin/keys`)

```bash
curl -X POST http://localhost:8002/v1/admin/keys \
  -H "Authorization: Bearer admin-secret-key-for-local-dev" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production AI Agent",
    "tenant_id": "dept-risk-01",
    "service_id": "ai-engine",
    "scopes": ["llm:*", "mcp:*"],
    "rate_limit_rpm": 600,
    "monthly_quota": 100000
  }'
```

**レスポンス**:
```json
{
  "key_id": "55d20ba0-d2e7-495b-a1c9-af33bfdf8f55",
  "key_prefix": "tlge-live-ca20",
  "name": "Production AI Agent",
  "tenant_id": "dept-risk-01",
  "service_id": "ai-engine",
  "scopes": ["llm:*", "mcp:*"],
  "rate_limit_rpm": 600,
  "monthly_quota": 100000,
  "status": "active",
  "raw_key": "tlge-live-ca20ceeb4561aef616268e2716ebb264"
}
```
> [!IMPORTANT]
> `raw_key` はキー生成時に 1 度だけ返却されます。安全なシークレットストアへ保管してください。

### ② ゲートウェイ経由のリクエスト転送

発行した API キーを `Authorization: Bearer <raw_key>` に指定してリクエストを送信する。

```bash
curl -X POST http://localhost:8002/mcp/v1/tools/execute \
  -H "Authorization: Bearer tlge-live-ca20ceeb4561aef616268e2716ebb264" \
  -H "Content-Type: application/json" \
  -d '{"tool_name": "slack_send_message", "arguments": {"channel": "#general", "text": "Hello!"}}'
```

Tollgate がキー検証・RPM レートリミット消費・クォータ加算を実行し、Prefix (`/mcp`) を除去した上で `http://mcp-gateway:8000/v1/tools/execute` へ転送。下流サービスへ `X-Tenant-ID: dept-risk-01` を自動注入する。

---

## テスト実行

```bash
# 全テスト実行
go test -v ./...

# キャッシュを無効化して実行
go test -v -count=1 ./...

# レースコンディション検出付きテスト
go test -race ./...
```

---

## 詳細仕様書 & ガイド

- [下流サービス連携・テナント解決・セキュリティ仕様 (docs/backend_integration.md)](docs/backend_integration.md)
- [コントリビューションガイド (CONTRIBUTING.md)](CONTRIBUTING.md)
- [セキュリティポリシー (SECURITY.md)](SECURITY.md)

---

## ライセンス

本プロジェクトは [MIT License](LICENSE) の下で公開されています。
