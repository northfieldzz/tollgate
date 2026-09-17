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

---

## 起動方法

### nerdctl compose を使用した起動

DynamoDB Local と初期テーブル作成コンテナ、DynamoDB Admin、Tollgate を一括起動する。

```bash
nerdctl compose -f compose.yaml -f dynamodb.compose.yaml up -d --build
```

- **Tollgate API**: `http://localhost:8002`
- **DynamoDB Admin (Web UI)**: `http://localhost:8003`

### 停止

```bash
nerdctl compose -f compose.yaml -f dynamodb.compose.yaml down
```

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

---

## テスト

コンテナイメージのビルド時にユニットテストが自動実行される。

```bash
nerdctl build -t tollgate:test .
```
