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
- **カラム統計** — `d` キーでカラムごとのオーバーレイ表示。NULL 率・distinct 数・min/max に加え、日付カラムにはスパークライン、数値カラムにはヒストグラム
- **Bring & Join** — `b` で現在の結果をローカル SQLite に持ち寄り、`J` で切替。異なる DB から持ち寄った結果同士を JOIN できる
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
| `d` | NORMAL | カラム統計オーバーレイを開く |
| `j` / `k` | STATS | カラムを移動 |
| `q` / `Esc` | STATS | カラム統計を閉じる |
| `Enter` | NORMAL | 現在行の詳細表示を開く |
| `PgUp` / `PgDn` | NORMAL | ページ移動 |
| `t` | NORMAL | テーブルサイドバーを開く |
| `S` | NORMAL | 保存クエリ（スニペット）を開く |
| `P` | NORMAL | 接続プロファイルを開く |
| `x` | PROFILE | 接続を切替して現在クエリを再実行 |
| `e` | NORMAL | エクスポートメニューを開く |
| `Ctrl+K` | NORMAL | AI アシスタントを開く |
| `b` | NORMAL | 現在の結果をローカルSQLite（Bring & Join）に持ち寄る |
| `J` | NORMAL | ローカルの Bring & Join DB に接続を切替 |
| `Ctrl+C` | *全モード* | 実行中のクエリ/AI をキャンセル、または終了 |
| `q` | NORMAL | 終了 |

## カラム統計

NORMAL モードで `d` を押すと統計オーバーレイが開きます。手元にある結果行からメモリ上で計算するため、追加のクエリは発行しません。

| 項目 | 意味 |
|------|------|
| `NULL%` | SQL NULL である行の割合 |
| `Distinct` | NULL を除いた異なり数 |
| `Min → Max` | 最小値と最大値（数値の場合は数値として比較） |

カーソル位置のカラムには、さらに1行が表示されます。

- **スパークライン**（日付・タイムスタンプ列）— 年/月/日のいずれかで件数をバケット化。例: `▁▂▃▅▇▅▂▁  by month`
- **ヒストグラム**（数値列）— 値の分布と対象レンジ。例: `▁▂▅█▇▃▁  0–100`

いずれも 10,000 行を超える場合は、オーバーレイの即時性を保つためスキップされます。

## Bring & Join

asql は異種 DB を直接統合しません。代わりにデータを手元へ持ち寄ります。どこかでクエリを実行し、その結果をローカル SQLite にコピーして、そこで JOIN します。

1. 1つ目の接続でクエリを実行し `b` を押す — 結果がローカルテーブル `t1` になります
2. 接続を切り替え（`P`）、別のクエリを実行して再度 `b` — こちらが `t2` になります
3. `J` でローカル DB に切り替え、またいでクエリします

```sql
SELECT t1.name, t2.score
FROM t1 JOIN t2 ON t1.id = t2.id;
```

コピーの際に値の型は保たれます。数値は数値のまま保存されるため、文字列比較ではなく数値としてソート・JOIN され、SQL NULL は文字列 `NULL` と区別されたままです。

持ち寄りの履歴はローカル DB 内の `_asql_bring` テーブルに記録されるため、「どこから持ってきたか」を後からいつでも確認できます。

```sql
SELECT * FROM _asql_bring;
```

| カラム | 意味 |
|--------|------|
| `n` | 持ち寄り順（`t1`, `t2`, … の番号と一致） |
| `table_name` | ローカルのテーブル名 |
| `source` | 取得元の接続名 |
| `row_count` / `col_count` | 持ち寄った結果のサイズ |
| `truncated` | 取得元の結果が 10,000 行の上限に達していた場合 `1`。そのテーブルは部分ビューです |
| `query` | データを生成したクエリ |

ローカル DB はメモリ上にあり、asql の終了とともに消えます。残したいものは `e` でエクスポートしてください。

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

環境変数は設定ファイルより優先されます。API キーをファイルに書きたくない場合はこちらを使ってください。

| 設定ファイル | 環境変数 |
|--------------|----------|
| `ai.ai_endpoint` | `ASQL_AI_ENDPOINT` |
| `ai.ai_model` | `ASQL_AI_MODEL` |
| `ai.ai_api_key` | `ASQL_AI_API_KEY` |

`ai_endpoint` と `ai_model` の両方が（ファイルまたは環境変数で）設定されている場合にのみ AI 機能が有効になります。

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
