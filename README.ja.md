[English](README.md)

# sqly

[Bubble Tea](https://github.com/charmbracelet/bubbletea) で構築されたターミナル UI SQL クライアント。現在は SQLite をサポートし、MySQL/PostgreSQL への対応を計画中。

## デモ

![sqly デモ](docs/demo.gif)

## インストール

[GitHub Releases](https://github.com/kwrkb/sqly/releases) からビルド済みバイナリをダウンロードできます。

Go でインストール:

```bash
go install github.com/kwrkb/sqly@latest
```

またはソースからビルド:

```bash
git clone https://github.com/kwrkb/sqly
cd sqly
go build -o sqly .
```

## 使い方

```bash
sqly <sqlite-ファイルパス>
```

### キーバインド

| キー | モード | 動作 |
|------|--------|------|
| `i` | NORMAL | INSERT モードに入る |
| `Esc` | INSERT | NORMAL モードに戻る |
| `Ctrl+Enter` / `Ctrl+J` | INSERT | クエリを実行 |
| `j` / `k` | NORMAL | 結果行を移動 |
| `t` | NORMAL | テーブルサイドバーを開く |
| `j` / `k` | SIDEBAR | テーブルを移動 |
| `Enter` | SIDEBAR | テーブルの SELECT クエリを挿入 |
| `Esc` / `t` | SIDEBAR | サイドバーを閉じる |
| `q` | NORMAL | 終了 |

## 開発

```bash
# テスト実行
go test ./...

# ビルド
go build

# Vet
go vet ./...
```

## ライセンス

MIT — [LICENSE](LICENSE) を参照
