[English](README.md)

# asql

**データ観察**のための軽量 TUI SQL クライアント — 生データを素早く見て、並べ替えて、探索し、違和感や仮説に気づくためのツール。[Bubble Tea](https://github.com/charmbracelet/bubbletea) で構築。SQLite、MySQL、PostgreSQL をサポート。

![asql デモ](docs/demo.gif)

## 設計思想

asql は分析基盤ではない — **データ観察ツール**である。

重い処理はクラウドでやればいい。asql はその手前、生データに素早く触れて違和感に気づき、仮説を立てる瞬間のためにある。軽く、静かで、手放せなくなる。

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
# SQLite
asql <sqlite-ファイルパス>

# MySQL
asql "mysql://user:password@host:3306/dbname"

# PostgreSQL
asql "postgres://user:password@host:5432/dbname"

# 保存済みプロファイルで接続
asql @myprofile

# 接続をプロファイルとして保存
asql --save-profile myprofile "postgres://user:pass@host:5432/db"

# 引数なし — 保存済みプロファイルから対話的に選択
asql

# ヘルプ / バージョン表示
asql --help
asql --version
```

## 主な機能

- **型情報付きヘッダ** — カラム名と型を並べて表示（`name text`、`age int`）
- **NULL / 空文字の区別** — NULL は `NULL`、空文字は `""` で表示し混同を防止
- **インプレースソート** — `s` キーでソート切替（None → Asc → Desc）、NULL は常に末尾
- **行詳細表示** — `Enter` でオーバーレイ表示、`j`/`k` でフィールド移動、`n`/`N` で行遷移
- **水平スクロール** — 多カラムテーブルを `h`/`l` で列単位スクロール、ステータスバーに `[3/12]` 表示
- **Tab 補完** — INSERT モードで `Tab` キーを押すと文脈に応じたテーブル名・カラム名を補完
- **クエリ履歴** — `Ctrl+P` / `Ctrl+N` で過去のクエリを呼び出し、`Ctrl+R` で履歴検索
- **保存クエリ（スニペット）** — `Ctrl+S` でクエリを保存、NORMAL モードで `S` でブラウズ
- **接続プロファイル** — DB 接続情報を保存・読込、NORMAL モードで `P` で切替
- **複数接続同時保持** — プロファイル切替時に既存接続を再利用、再接続のオーバーヘッドなし
- **横並び比較モード** — `c` キーで現在結果を左ペインに固定し、左（固定）/右（アクティブ）の2画面比較。`Tab` でフォーカス切替。件数差と不一致セルを即時ハイライト
- **接続をまたいだ高速再実行** — `R` キーで現在クエリを再実行。プロファイルモードで `x` を押すと接続切替と同時に再実行
- **ページング表示** — ステータスバーに現在位置とカラム情報を表示（`col:name 1/100`）
- **テーブルサイドバー** — テーブル一覧をブラウズし、ワンキーで SELECT を挿入
- **エクスポート** — CSV / JSON / Markdown でコピー、またはファイル保存
- **AI アシスタント** — OpenAI 互換 API で自然言語から SQL を生成

## 比較モード

![比較デモ](docs/compare-demo.gif)

> 本番とステージングの差分をターミナルで3秒で確認。

## キーバインド

| キー | モード | 動作 |
|------|--------|------|
| `i` | NORMAL | INSERT モードに入る |
| `Esc` | INSERT | NORMAL モードに戻る |
| `Ctrl+Enter` / `Ctrl+J` | INSERT | クエリを実行 |
| `Tab` | INSERT | テーブル名・カラム名を補完 |
| `Ctrl+P` / `Ctrl+N` | INSERT | クエリ履歴の前 / 次 |
| `Ctrl+R` | INSERT | クエリ履歴を検索 |
| `Ctrl+S` | INSERT | 現在のクエリをスニペットとして保存 |
| `Ctrl+L` | INSERT | エディタをクリア |
| `c` | NORMAL | 比較モードを切替（現在結果を固定 / 比較を終了） |
| `Tab` | NORMAL（比較中） | フォーカスペインを切替（左 / 右） |
| `j` / `k` | NORMAL | 結果行を移動 |
| `h` / `l` | NORMAL | カラムを水平スクロール |
| `s` | NORMAL | 選択カラムのソートを切替 |
| `R` | NORMAL | 現在のクエリを再実行 |
| `Enter` | NORMAL | 現在行の詳細表示を開く |
| `PgUp` / `PgDn` | NORMAL | ページ移動 |
| `t` | NORMAL | テーブルサイドバーを開く |
| `S` | NORMAL | 保存クエリ（スニペット）を開く |
| `P` | NORMAL | 接続プロファイルを開く |
| `x` | PROFILE | 接続を切替して現在クエリを再実行 |
| `e` | NORMAL | エクスポートメニューを開く |
| `Ctrl+K` | NORMAL | AI アシスタントを開く |
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

設定ファイルがない場合、AI 機能はサイレントに無効化されます。

## 開発

```bash
go test ./...
go build
go vet ./...
```

## ライセンス

MIT — [LICENSE](LICENSE) を参照
