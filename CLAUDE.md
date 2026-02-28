# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

asql は Go 製の TUI SQL クライアント。Bubble Tea (Charmbracelet) フレームワークで構築され、SQLite・MySQL・PostgreSQL をサポート。OpenAI 互換 API（Ollama / LM Studio）による自然言語→SQL 生成機能を搭載。

## Commands

```bash
# ビルド
go build

# 実行
./asql <sqlite-file-path>
./asql "mysql://user:pass@host:3306/dbname"
./asql "postgres://user:pass@host:5432/dbname"

# テスト
go test ./...

# 単一パッケージのテスト
go test ./internal/db/sqlite/
```

## Architecture

3層構造:

- **main.go** — エントリポイント。引数解析 → DB接続 → Bubble Tea起動
- **internal/db/** — データベース抽象層
  - `adapter.go`: `DBAdapter` インターフェース（`Type()`, `Query()`, `Tables()`, `Schema()`, `Close()`）と `QueryResult` 型
  - `dbutil/dbutil.go`: 共通ヘルパー（`StringifyValue()`, `ScanRows()`, `LeadingKeyword()`）
  - `sqlite/adapter.go`: SQLite 実装。`returnsRows()` は先頭キーワード + `containsReturning()` で判定
  - `mysql/adapter.go`: MySQL 実装。`mysql://` DSN を `go-sql-driver` 形式に変換。RETURNING 未対応
  - `postgres/adapter.go`: PostgreSQL 実装。`postgres://` URL をそのまま pgx に渡す。ドル引用符対応の `containsReturning()`
- **internal/ui/** — TUI層
  - `model.go`: Bubble Tea Model。textarea（エディタ）+ table（結果表示）+ viewport + ステータスバー。NORMAL/INSERT/SIDEBAR/AI/EXPORT の5モード切替。`queryCancel context.CancelFunc` でクエリ/AI実行中の Ctrl+C キャンセルに対応
  - `export.go`: エクスポートモードのキーバインド・実行ロジック・モーダル描画
- **internal/export/** — エクスポート層
  - `export.go`: CSV/JSON/Markdown フォーマット変換 + ファイル書き出し。UI/DB に依存しない純粋関数パッケージ
- **internal/ai/** — AI 層
  - `client.go`: OpenAI Chat Completions API クライアント。スキーマをシステムプロンプトに注入し SQL を生成
- **internal/config/** — 設定管理
  - `config.go`: `~/.config/asql/config.yaml` から AI 設定（endpoint, model, api_key）を読み込み

**設計ポイント**: `DBAdapter` インターフェースにより、UI層はDB実装に依存しない。新しいDBドライバは `internal/db/<driver>/adapter.go` に追加する。`main.go` が DSN prefix（`mysql://`, `postgres://`）で自動判定し適切なドライバに振り分ける。AI 機能はオプションで、設定未構成時はサイレントに無効化される。

## Dependencies

- `charmbracelet/bubbletea` + `bubbles` + `lipgloss` — TUIフレームワーク・スタイリング
- `modernc.org/sqlite` — Pure Go SQLite ドライバ（CGO不要）
- `github.com/go-sql-driver/mysql` — MySQL ドライバ
- `github.com/jackc/pgx/v5/stdlib` — PostgreSQL ドライバ（pgx）
- `gopkg.in/yaml.v3` — 設定ファイルパーサ
- `github.com/atotto/clipboard` — クリップボードアクセス（エクスポート機能）

## Workflow Files

### PLAN.md

- タスク開始時に作成し、完了後もリポジトリに残す
- フォーマットは下記テンプレートに従う
- 進捗は `[ ]` → `[x]` で更新する
- 想定外の問題が出たら「変更履歴」に記録して再計画する
- テンプレート:

  ```
  # <タスク名>

  ## 目的
  <何を・なぜやるか>

  ## タスク
  - [ ] ステップ1
  - [ ] ステップ2
  - [ ] ステップ3

  ## 検証
  - <どうやって完了を確認するか>

  ## 変更履歴
  - (想定外の変更が発生したら記録)
  ```

### LESSONS.md

- ユーザーから修正・指摘を受けたら必ずエントリを追加する
- 既存エントリと重複する場合は、既存エントリを更新する
- セッション開始時に内容を確認する
- エントリフォーマット:

  ```
  ## カテゴリ名

  ### 教訓タイトル（簡潔に）

  **文脈**: 何が起きたか（バグ・指摘の状況）
  **学び**: 根本原因と理解すべきこと
  **パターン**: 正しいコード例や判断基準
  ```
