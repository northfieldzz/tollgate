# Tollgate 連携バックエンドサービス実装仕様書 (Backend Integration Guide)

本書は、Tollgate のリバースプロキシ配下に配置される下流バックエンドサービス（例: `llm-gateway`, `mcp-gateway`, `ai-engine` 等）が、Tollgate から転送されるリクエストを処理するにあたって参照すべきヘッダー仕様および実装要件をまとめたドキュメントである。

---

## 1. アーキテクチャ概要

```
[Client]
   │  Authorization: Bearer <API_KEY> または X-API-Key
   ▼
[Tollgate (Reverse Proxy / API Gateway)]
   │  1. API キーの妥当性検証・失効/有効期限チェック
   │  2. スコープ (Scope) 認可チェック
   │  3. レートリミット (RPM) / 月間クォータ (Quota) 消費・判定
   │  4. 認証コンテキストヘッダーの注入 (X-Tenant-ID 等)
   ▼
[Backend Service (llm-gateway / mcp-gateway / ai-engine / etc.)]
      `X-Tenant-ID` に基づいてテナント固有リソースへのアクセスを制御
```

Tollgate はリクエストを受信すると、API キーの検証・レート制限・スコープ確認を行い、検証にパスしたリクエストのみを下流バックエンドサービスへ転送する。

---

## 2. 転送リクエストヘッダー仕様

Tollgate での検証成功時、バックエンドサービスへ転送される HTTP リクエストヘッダーに以下の認証コンテキスト情報が注入される。

| ヘッダー名 | 必須性 | 型 / フォーマット | 説明 |
|---|---|---|---|
| `X-Tenant-ID` | **必須 (常に付与)** | 文字列 (UUID / 識別子) | **テナント ID**。下流サービスでのマルチテナントデータ分離・アクセスコントロールのキーとして使用する。<br>・**テナントキー**: キー固定の `tenant_id` を付与（クライアント指定値と不一致の場合は `403 Forbidden` で遮断）。<br>・**サービスキー**: クライアントが指定した動的 `X-Tenant-ID` を透過フォワード（未指定時は `400 Bad Request`）。 |
| `X-Key-ID` | **必須 (常に付与)** | 文字列 (UUID) | 使用された API キーの一意な ID。監査ログやキー単位の利用状況追跡に使用する。 |
| `X-Key-Prefix` | **必須 (常に付与)** | 文字列 (例: `tlge-live-8f9c`) | API キーの先頭プレフィックス識別子。ログ出力やトラブルシューティングでのキー特定に使用する。 |
| `X-Service-ID` | 任意 (条件付き付与) | 文字列 | API キー発行時に `service_id` が設定されていた場合のみ付与される。特定サービス専用キーの識別に利用可能。 |

### 2.1 テナント解決 & コンフリクト検証ルール

Tollgate リバースプロキシは、クライアントからのヘッダースプーフィング防止とマルチテナント透過連携を両立するため、以下のロジックでテナントヘッダーを制御・検証する：

1. **テナントキー（API キーに `tenant_id` が設定されている場合）**:
   - クライアントが `X-Tenant-ID` を**指定しない**場合：キーの `tenant_id` を自動注入して下流へ転送。
   - クライアントが `X-Tenant-ID` を**指定し、キーと一致**する場合：正常通過。
   - クライアントが `X-Tenant-ID` を**指定し、キーと不一致**の場合：**`403 Forbidden` (`tenant_mismatch`)** で即座にリクエストを拒絶（なりすまし・設定ミスの防止）。
2. **サービスキー（API キーに `tenant_id` が未設定、`service_id` が設定されている場合）**:
   - クライアントが `X-Tenant-ID` を**指定**した場合：クライアント指定値をそのまま透過フォワード。キーの `service_id` を `X-Service-ID` に注入。
   - クライアントが `X-Tenant-ID` を**未指定**の場合：**`400 Bad Request` (`missing_tenant_id`)** で拒絶（エンド顧客特定不可）。

### ヘッダーの具体例

```http
POST /tools/list HTTP/1.1
Host: mcp-gateway:8000
X-Tenant-ID: tenant_corp_abc123
X-Key-ID: 550e8400-e29b-41d4-a716-446655440000
X-Key-Prefix: tlge-live-8f9c
X-Service-ID: svc-mcp-cluster-1
Content-Type: application/json
...
```

---

## 3. バックエンドサービス側の実装要件

### 3.1 テナントコンテキストの取得
- リクエストヘッダー `X-Tenant-ID` を取得し、後続の DB クエリやストレージアクセス時のパーティションキー / テナント分離キーとして必ず適用すること。
- ヘッダーが存在しない、または空の場合は `401 Unauthorized` または `403 Forbidden` を返却する設計を推奨する（Tollgate 経由外からの不正アクセス防止）。

### 3.2 API キーの再検証は不要
- Tollgate を通過した時点で以下の検証は完了しているため、バックエンドサービス側で API キー自体の照合や DB 参照を行う必要はない。
  - API キーのハッシュ照合・存在確認
  - キーの失効（Revoke）/ 一時停止（Suspend）/ 有効期限チェック
  - ルートに設定されたスコープ合致（Fast-Fail）
  - 秒間/分間レートリミット（RPM）および月間クォータの残量判定

### 3.3 監査ログへの記録
- ログを出力する際は、トレーサビリティ向上のため `tenant_id` (`X-Tenant-ID`) および `key_id` (`X-Key-ID`) を構造化ログのフィールドに含めることを推奨する。

### 3.4 パスプレフィックスの考慮
- ルート設定で `strip_prefix: true` が指定されている場合、Tollgate でプレフィックスが除去された状態でバックエンドに到達する。
  - 例: クライアントが `POST /mcp/v1/tools` へリクエスト → バックエンドには `POST /v1/tools` として到達。
  - バックエンド側のルーター定義では、除去後のパス（例: `/v1/tools`）をリッスンするように構築する。

---

## 4. セキュリティ注意事項

> [!WARNING]
> **ヘッダー偽装防止のためのネットワーク分離**
> クライアントが直接バックエンドサービスのポートへアクセスできるネットワーク構成になっている場合、クライアント自身が `X-Tenant-ID` ヘッダーを偽装してなりすましを行うリスクがある。
> バックエンドサービスは内部プライベートネットワーク（VPC / Docker 内部ネットワーク）内に配置し、**Tollgate からのインバウンド通信のみを許可**すること。

なお、Tollgate 経由でリクエストが送られた場合、Tollgate はクライアントから送られてきた既存の `X-Tenant-ID` などのヘッダーを上書き (`r.Header.Set`) して転送するため、プロキシ経由時のヘッダー改ざんは防止される。

---

## 5. 参考: クライアントに返却されるレートリミットヘッダー

Tollgate はリクエスト検証時、クライアント向けのレスポンスヘッダーに流量情報を付与する。バックエンドサービス側で独自に付与する必要はない。

| レスポンスヘッダー名 | 説明 |
|---|---|
| `X-RateLimit-Limit-RPM` | キーに設定された 1 分あたりのリクエスト上限数 |
| `X-RateLimit-Remaining-RPM` | 当該スライディングウィンドウにおける残りリクエスト可能数 |
| `X-Quota-Limit` | キーに設定された月間クォータ上限数（クォータ無制限時は付与されない） |
| `X-Quota-Remaining` | 当月の残りクォータ数 |

---

## 6. 参考: スタンドアロン検証 API (`POST /v1/verify`) 仕様

プロキシモードを使用せず、各マイクロサービスが独自に Tollgate へ検証リクエストを行うアーキテクチャを採用する場合は、以下の API を呼び出して判定結果を取得する。

### リクエスト
`POST /v1/verify`
```json
{
  "raw_key": "tlge-live-8f9c2d1e0a4b3c5d6e7f8a9b0c1d2e3f",
  "required_scope": "mcp:tools:execute"
}
```

### レスポンス (`200 OK`)
```json
{
  "valid": true,
  "key_id": "550e8400-e29b-41d4-a716-446655440000",
  "key_prefix": "tlge-live-8f9c",
  "tenant_id": "tenant_corp_abc123",
  "service_id": "svc-mcp-cluster-1",
  "scopes": ["mcp:*"],
  "remaining_rpm": 59,
  "limit_rpm": 60,
  "remaining_quota": 99500,
  "monthly_quota": 100000
}
```
※検証失敗時は `valid: false` となり、`reason`（例: `invalid_key`, `suspended`, `expired`, `scope_mismatch`, `rate_limit_exceeded`, `quota_exceeded`）が返却される。
