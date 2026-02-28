# AI アシスタント機能（Text-to-SQL）追加

## 目的
自然言語から SQL を生成する AI 機能を追加。Ollama / LM Studio（OpenAI 互換 API）をバックエンドとして利用。

## タスク
- [x] `internal/config/config.go` — Config / AIConfig 構造体、Load()、AIEnabled()
- [x] `internal/config/config_test.go` — ファイル不在/正常/不正YAML/部分設定のテスト
- [x] `internal/db/adapter.go` — Schema(context.Context) を DBAdapter に追加
- [x] `internal/db/sqlite/adapter.go` — Schema 実装（sqlite_master から CREATE TABLE 取得）
- [x] `internal/db/sqlite/adapter_test.go` に TestSchema 追加
- [x] `internal/ai/client.go` — OpenAI 互換 API クライアント + stripCodeFences
- [x] `internal/ai/client_test.go` — httptest で正常/エラー/空レスポンス/コードフェンス除去テスト
- [x] `internal/ui/model.go` — aiMode、モーダル、spinner、Ctrl+K ハンドラ
- [x] `main.go` — config 読み込み、AI クライアント生成、NewModel 更新
- [x] `go.mod` — yaml.v3 追加

## 検証
- `go build` 成功 ✅
- `go test ./...` 全 pass ✅
- 設定ファイルなしで起動 → AI 無効で従来通り動作
