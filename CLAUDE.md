# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

sqly は Go 製の TUI SQL クライアント。Bubble Tea (Charmbracelet) フレームワークで構築され、現在は SQLite をサポート。将来的に MySQL/PostgreSQL への拡張を想定した設計。

## Commands

```bash
# ビルド
go build

# 実行
./sqly <sqlite-file-path>

# テスト
go test ./...

# 単一パッケージのテスト
go test ./internal/db/sqlite/
```

## Architecture

3層構造:

- **main.go** — エントリポイント。引数解析 → DB接続 → Bubble Tea起動
- **internal/db/** — データベース抽象層
  - `adapter.go`: `DBAdapter` インターフェース（`Query()`, `Close()`）と `QueryResult` 型
  - `sqlite/adapter.go`: SQLite 実装。
    - `returnsRows()`: 先頭キーワード（SELECT/PRAGMA/WITH/EXPLAIN/VALUES）か、DML に `RETURNING` 句があるかで判定
    - `leadingKeyword()`: コメント・セミコロンをスキップして先頭キーワードを返す
    - `containsReturning()`: 文字列リテラル・引用符識別子・コメントをスキップしつつ `RETURNING` キーワードを単語境界で検出
    - `stringifyValue()`: NULL, []byte（UTF-8有効→文字列、無効→hex）, time.Time, その他に対応
- **internal/ui/** — TUI層
  - `model.go`: Bubble Tea Model。textarea（エディタ）+ table（結果表示）+ viewport + ステータスバー。NORMAL/INSERT の2モード切替

**設計ポイント**: `DBAdapter` インターフェースにより、UI層はDB実装に依存しない。新しいDBドライバは `internal/db/<driver>/adapter.go` に追加する。

## Dependencies

- `charmbracelet/bubbletea` + `bubbles` + `lipgloss` — TUIフレームワーク・スタイリング
- `modernc.org/sqlite` — Pure Go SQLite ドライバ（CGO不要）
