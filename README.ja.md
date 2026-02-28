[English](README.md)

# asql

[Bubble Tea](https://github.com/charmbracelet/bubbletea) で構築されたターミナル UI SQL クライアント。現在は SQLite をサポートし、MySQL/PostgreSQL への対応を計画中。OpenAI 互換 API（Ollama、LM Studio 等）を利用した自然言語からの SQL 生成機能を搭載。

## デモ

![asql デモ](docs/demo.gif)

## インストール

[GitHub Releases](https://github.com/kwrkb/asql/releases) からビルド済みバイナリをダウンロードできます。

Go でインストール:

```bash
go install github.com/kwrkb/asql@latest
```

またはソースからビルド:

```bash
git clone https://github.com/kwrkb/asql
cd asql
go build -o asql .
```

## 使い方

```bash
asql <sqlite-ファイルパス>
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
| `e` | NORMAL | エクスポートメニューを開く |
| `j` / `k` | EXPORT | 選択肢を移動 |
| `Enter` | EXPORT | エクスポート実行 |
| `Esc` | EXPORT | キャンセル |
| `Ctrl+K` | NORMAL | AI アシスタントを開く |
| `Enter` | AI | SQL を生成 |
| `Esc` | AI | キャンセル |
| `Ctrl+C` | *全モード* | 実行中のクエリ/AI をキャンセル、または終了 |
| `q` | NORMAL | 終了 |

## エクスポート

クエリ実行後、NORMAL モードで `e` を押すとエクスポートメニューが開きます。対応フォーマット:

- **Copy as CSV** — クリップボードにコピー
- **Copy as JSON** — クリップボードにコピー（オブジェクト配列）
- **Copy as Markdown** — クリップボードにコピー（GFM テーブル）
- **Save to File (CSV)** — カレントディレクトリに `result_YYYYMMDD_HHMMSS.csv` を保存

## AI アシスタント（Text-to-SQL）

OpenAI 互換 API を利用して、自然言語から SQL を生成できます。`~/.config/asql/config.yaml` に設定ファイルを作成してください:

```yaml
ai:
  ai_endpoint: http://localhost:11434/v1   # Ollama
  ai_model: llama3
  ai_api_key: ""                           # 省略可（Ollama は不要）
```

NORMAL モードで `Ctrl+K` を押すと AI プロンプトが開きます。データベースのスキーマ情報が自動的にコンテキストに含まれるため、正確なテーブル名・カラム名で SQL が生成されます。

設定ファイルがない場合、AI 機能はサイレントに無効化され、従来通り動作します。

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
