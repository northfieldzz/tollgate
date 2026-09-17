# Tollgate

マルチテナント対応の API キー管理（発行・失効・ローテーション）およびリクエストレートリミット / クォータ検証ゲートウェイサービス。

---

## 主な機能

- **API キーライフサイクル管理**:
  - API キーの新規発行（有効期限、レートリミット、月間クォータ、許可 IP 等の設定）
  - テナント単位の API キー一覧取得
  - API キーの即時失効（Revoke）
  - グレースピリオド（猶予期間）付きキーローテーション
- **検証 & 流量制御**:
  - API キーの妥当性検証（ハッシュ照合、有効期限、ステータス）
  - スライディングウィンドウ方式による秒間レートリミット（RPS）制御
  - 月間クォータ制限の検証
  - CIDR による IP アドレスホワイトリスト検証
- **オブザーバビリティ**:
  - Prometheus メトリクスエンドポイント (`/metrics`)
  - Liveness / Readiness ヘルスチェックエンドポイント (`/health/live`, `/health/ready`)
  - OpenAPI 3.1 スキーマ提供 (`/openapi`, `/openapi.json`)

---

## アーキテクチャ構成

```
tollgate/
├── cmd/
│   └── server/          # エントリーポイント (main.go)
├── internal/
│   ├── delivery/
│   │   └── http/        # HTTP ハンドラー & ルーティング (Huma v2)
│   ├── domain/
│   │   ├── entity/      # ドメインエンティティ (APIKey, Tenant など)
│   │   └── repository/  # リポジトリインターフェース
│   ├── infrastructure/
│   │   ├── dynamodb/    # DynamoDB クライアント & リポジトリ実装
│   │   ├── metrics/     # Prometheus メトリクス実装
│   │   └── ratelimit/   # スライディングウィンドウ・レートリミッター
│   └── usecase/         # ビジネスロジック (KeyUsecase, VerifyUsecase)
├── compose.yaml         # Tollgate サービス定義
├── dynamodb.compose.yaml # DynamoDB Local / Admin / Init 定義
└── Dockerfile           # マルチステージビルド定義
```

---

## 開発環境のセットアップ

### 前提条件

- [nerdctl](https://github.com/containerd/nerdctl) 2.x 以上

### 環境変数設定

`.env.example` をコピーして `.env` を作成する。

```bash
cp .env.example .env
```

| 変数名 | デフォルト値 | 説明 |
|---|---|---|
| `PORT` | `8000` | Tollgate サーバーのポート番号 |
| `DYNAMODB_ENDPOINT` | `http://dynamodb:8000` | DynamoDB の接続エンドポイント |
| `AWS_REGION` | `ap-northeast-1` | AWS リージョン |
| `TABLE_NAME` | `ITCP_APIKeys` | API キー格納先テーブル名 |
| `PROXY_ROUTES` | *(任意)* | 動的ルート定義 (JSON 配列文字列)。後述の仕様を参照 |
| `ROUTES_CONFIG_FILE` | *(任意)* | 動的ルート定義ファイルのパス (例: `./routes.json`) |
| `FORWARD_TARGET_URL` | *(任意)* | ルート未マッチ時のデフォルトフォールバック転送先 (例: `http://webapi:8000`) |

---

## 起動方法

### nerdctl compose を使用した起動

DynamoDB Local と初期テーブル作成コンテナ、DynamoDB Admin、Tollgate を一括起動する。

```bash
nerdctl compose -f compose.yaml -f dynamodb.compose.yaml up -d --build
```

- **Tollgate API / Gateway**: `http://localhost:8002`
- **DynamoDB Admin (Web UI)**: `http://localhost:8003`

### 停止

```bash
nerdctl compose -f compose.yaml -f dynamodb.compose.yaml down
```

---

## 動作モード & ルーティング仕様

### 1. 動的マルチターゲット・リバースプロキシモード

リクエストパスのプレフィックスに応じて、複数のバックエンドサービスへ自動ルーティングする。

#### ルーティングの定義方法

JSON 形式で各サービスのルーティングを定義する。環境変数 `PROXY_ROUTES` または設定ファイル `ROUTES_CONFIG_FILE` で指定可能。

```json
[
  {
    "prefix": "/llm",
    "target": "http://llm-gateway:8000",
    "scope": "llm:*",
    "strip_prefix": true
  },
  {
    "prefix": "/mcp",
    "target": "http://mcp-gateway:8000",
    "scope": "mcp:*",
    "strip_prefix": true
  },
  {
    "prefix": "/ai",
    "target": "http://ai-engine:8000",
    "scope": "ai:*",
    "strip_prefix": true
  }
]
```

- **環境変数で指定する場合 (`PROXY_ROUTES`)**:
  1 行の JSON 文字列として指定。
- **設定ファイルで指定する場合 (`ROUTES_CONFIG_FILE`)**:
  外部マウントした `routes.json` などのファイルパスを指定。


#### プレフィックス除去（StripPrefix）

`strip_prefix: true` の場合、下流サービスにはプレフィックスを除去したパスが渡される。
- クライアントリクエスト: `POST /mcp/tools/list?filter=active`
- 下流バックエンド転送: `POST /tools/list?filter=active`

#### 認可（スコープ Fast-Fail）

ルートに `scope`（例: `mcp:*`）が設定されている場合、API キーの許可スコープと照合される。
- スコープを満たさないキーでのリクエストは **`403 Forbidden` (`reason: scope_mismatch`)** を即時返却し、バックエンドには一切リクエストを流さない。

#### コンテキスト情報の自動付与

検証成功時、バックエンドに以下のヘッダーを自動付与して転送:
- `X-Tenant-ID`: テナント ID
- `X-Key-ID`: キー ID
- `X-Key-Prefix`: キープレフィックス
- `X-Service-ID`: サービス ID

#### ストリーミング対応

- **SSE (Server-Sent Events)**: `FlushInterval = -1` によりバッファリングなしで即座にリアルタイム中継。
- **WebSocket**: `Connection: Upgrade` を透過。

---

### 2. スタンドアロン API モード

プロキシ環境変数が一切設定されていない場合、キー管理 API および明示的検証 API (`/verify`) のみを提供する。

---

## API エンドポイント一覧

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/health/live` | Liveness ヘルスチェック |
| `GET` | `/health/ready` | Readiness ヘルスチェック (DynamoDB 接続確認) |
| `GET` | `/metrics` | Prometheus メトリクス |
| `GET` | `/openapi` | OpenAPI 3.1 スキーマ (JSON) |
| `GET` | `/openapi.json` | OpenAPI 3.1 スキーマ (JSON) |
| `POST` | `/keys` | 新規 API キー発行 |
| `GET` | `/keys` | テナントの API キー一覧取得 |
| `DELETE` | `/keys/{key_id}` | API キーの失効 |
| `POST` | `/keys/{key_id}/rotate` | API キーのローテーション |
| `POST` | `/verify` | API キー検証および流量制御判定 |
| `ANY` | `/llm/*` | **(プロキシ)** `llm_gateway` へ透過転送 (StripPrefix: `/llm`) |
| `ANY` | `/mcp/*` | **(プロキシ)** `mcp_gateway` へ透過転送 (StripPrefix: `/mcp`) |
| `ANY` | `/ai/*` | **(プロキシ)** `ai_engine` へ透過転送 (StripPrefix: `/ai`) |
| `ANY` | `/*` | **(動的プロキシ)** `PROXY_ROUTES` 定義に基づく転送 |

---

## テスト

コンテナイメージのビルド時にユニットテストが自動実行される。

```bash
nerdctl build -t tollgate:test .
```

---

## ライセンス

[MIT License](LICENSE)

