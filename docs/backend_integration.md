# Tollgate 連携バックエンドサービス実装仕様書 (Backend Integration Guide)

本書は、Tollgate のリバースプロキシ配下に配置される下流バックエンドサービス（例: `users-service`, `billing-service`, `analytics-service` 等）が、Tollgate から転送されるリクエストを処理するにあたって参照すべきヘッダー仕様および実装要件をまとめたドキュメントである。

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
   │  4. 認証コンテキストヘッダーの注入 (X-Tenant-ID / X-Service-ID 等)
   ▼
[Backend Service (users-service / billing-service / etc.)]
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
| `X-Service-ID` | **必須 (常に付与)** | 文字列 | 呼び出し元サービス識別子。API キー発行時に設定された `service_id` が常に付与される（クライアント指定値と不一致の場合は `403 Forbidden` で遮断）。 |

### 2.1 認証コンテキスト解決 & コンフリクト検証ルール

Tollgate リバースプロキシは、クライアントからのヘッダースプーフィング防止とマルチテナント透過連携を両立するため、以下のロジックでヘッダーを制御・検証する：

1. **テナントキー（API キーに `tenant_id` と `service_id` が設定されている場合）**:
   - クライアントが `X-Tenant-ID` を**指定しない**場合：キーの `tenant_id` を自動注入して下流へ転送。
   - クライアントが `X-Tenant-ID` を**指定し、キーと一致**する場合：正常通過。
   - クライアントが `X-Tenant-ID` を**指定し、キーと不一致**の場合：**`403 Forbidden` (`tenant_mismatch`)** で即座にリクエストを拒絶（なりすまし・設定ミスの防止）。
   - キーの `service_id` を `X-Service-ID` に自動注入（クライアント指定値と不一致の場合は **`403 Forbidden` (`service_mismatch`)** で拒絶）。
2. **サービスキー（API キーに `tenant_id` が未設定、`service_id` が設定されている場合）**:
   - クライアントが `X-Tenant-ID` を**指定**した場合：クライアント指定値をそのまま透過フォワード。キーの `service_id` を `X-Service-ID` に自動注入（クライアント指定値と不一致の場合は **`403 Forbidden` (`service_mismatch`)** で拒絶）。
   - クライアントが `X-Tenant-ID` を**未指定**の場合：**`400 Bad Request` (`missing_tenant_id`)** で拒絶（エンド顧客特定不可）。

### ヘッダーの具体例

```http
POST /v1/invoices HTTP/1.1
Host: billing-service:8000
X-Tenant-ID: tenant_corp_abc123
X-Key-ID: 550e8400-e29b-41d4-a716-446655440000
X-Key-Prefix: tlge-live-8f9c
X-Service-ID: billing-service
Content-Type: application/json
```

---

## 3. バックエンドサービスの実装要件

### 3.1 テナントコンテキストの取得
- リクエストヘッダー `X-Tenant-ID` を取得し、後続の DB クエリ・ストレージアクセスのパーティションキー / テナントキーとして必ず適用すること。
- ヘッダーが存在しない、または空の場合は `401 Unauthorized` または `403 Forbidden` を返却する設計を推奨（Tollgate 経由外の不正アクセス防止）。

### 3.2 API キーの再検証は不要
- Tollgate を通過した時点で以下の検証は完了しているため、バックエンドサービス側で API キー自体の照合や DB 参照を行う必要はない。
  - API キーのハッシュ照合・存在確認
  - キーの失効（Revoke）/ 一時停止（Suspend）/ 有効期限チェック
  - ルートに設定されたスコープの充足確認（Fast-Fail）
  - 分間レートリミット（RPM）および月間クォータの残数判定・消費

### 3.3 監査ログへの記録
- ログ出力時は、トレーサビリティ向上のため `tenant_id` (`X-Tenant-ID`) と `key_id` (`X-Key-ID`) を構造化ログのフィールドに含めることを推奨。

### 3.4 パスプレフィックスの考慮
- ルート設定で `strip_prefix: true` が指定されている場合、Tollgate 側でプレフィックスが除去された状態でバックエンドに到達する。
  - 例: クライアントが `POST /billing/v1/invoices` へリクエスト → バックエンドには `POST /v1/invoices` として到達。
  - バックエンド側のルーター定義では、除去後のパス（例: `/v1/invoices`）にマッチするように構築する。

---

## 4. セキュリティ注意事項

> [!WARNING]
> **ヘッダー偽装防止のためのネットワーク隔離**
> クライアントから直接バックエンドサービスのポートへアクセスできるネットワーク構成になっている場合、クライアント自身が `X-Tenant-ID` ヘッダーを偽装してなりすましを行うリスクが生じる。
> バックエンドサービスは内部プライベートネットワーク（VPC / Docker 内部ネットワーク）に配置し、**Tollgate からのインバウンド通信のみを許可**すること。

なお、Tollgate 経由でリクエストされた場合、Tollgate はクライアントから送られてきた `X-Tenant-ID` などのヘッダーを上書き・検証 (`r.Header.Set`) して転送するため、プロキシ経由のヘッダー偽装は防止される。

---

## 5. 参考: クライアントに返却されるレートリミットヘッダー

Tollgate はリクエスト処理後、クライアントへのレスポンスヘッダーに流量情報を付与する。バックエンドサービス側で独自に付与する必要はない。

| レスポンスヘッダー名 | 説明 |
|---|---|
| `X-RateLimit-Limit-RPM` | キーに設定された 1 分あたりのリクエスト上限数 |
| `X-RateLimit-Remaining-RPM` | 当該スライディングウィンドウにおける残りリクエスト可能数 |
| `X-Quota-Limit` | キーに設定された月間クォータ上限数（クォータ無制限時は付与されない） |
| `X-Quota-Remaining` | 当月の残りクォータ数 |

---

## 6. 参考: スタンドアロン検証 API (`POST /v1/admin/verify`) 仕様

プロキシモードを使用せず、各マイクロサービスが独自に Tollgate へ検証リクエストを行うアーキテクチャを採用する場合は、以下の API を呼び出して判定結果を取得する（`Authorization: Bearer <ADMIN_API_KEY>` が必須）。

### リクエスト
`POST /v1/admin/verify`
```http
POST /v1/admin/verify HTTP/1.1
Authorization: Bearer <ADMIN_API_KEY>
Content-Type: application/json

{
  "raw_key": "tlge-live-8f9c2d1e0a4b3c5d6e7f8a9b0c1d2e3f",
  "required_scope": "billing:invoices:write"
}
```

### レスポンス (`200 OK`)
```json
{
  "valid": true,
  "key_id": "550e8400-e29b-41d4-a716-446655440000",
  "key_prefix": "tlge-live-8f9c",
  "tenant_id": "tenant_corp_abc123",
  "service_id": "billing-service",
  "scopes": ["billing:*"],
  "remaining_rpm": 59,
  "limit_rpm": 60,
  "remaining_quota": 99500,
  "monthly_quota": 100000
}
```
検証失敗時は `valid: false` となり、`reason`（例: `invalid_key`, `suspended`, `expired`, `scope_mismatch`, `rate_limit_exceeded`, `quota_exceeded`）が返却される。
