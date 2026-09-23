---
name: バグ報告 (Bug Report)
about: 不具合や予期しない動作の報告
title: '[BUG] '
labels: 'bug'
assignees: ''
---

## 不具合の概要
発生している不具合の簡潔な説明を記入してください。

## 再現手順
不具合を再現するための具体的な手順：
1. 起動設定・環境変数: `DB_BACKEND=...`, `RATE_LIMIT_BACKEND=...`
2. 送信したリクエスト: `curl -X POST ...`
3. 発生したレスポンス / エラーログ:

## 期待される動作
本来期待される正しい動作やレスポンスを記入してください。

## 実行環境情報
- Tollgate バージョン / コミットハッシュ: 
- OS: [例: Linux (Ubuntu 22.04), macOS, Windows]
- データベースバックエンド: [SQLite / PostgreSQL / DynamoDB]
- レートリミットバックエンド: [In-Memory / Redis / DynamoDB]
- Go バージョン (ローカル実行時): [例: go1.24.0]

## 補足情報・ログ
追加のコンテキストやサーバーログがあれば貼り付けてください。
