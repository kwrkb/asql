# NOTES.md — 内部構造の索引

**揮発・鮮度非保証。** このファイルは「どこを読めばいいか」の道案内であり、仕様ではない。
コードと食い違っていたら**常にコードが正しい**（優先順位: VISION.md > PLAN.md > コード > NOTES.md）。
気づいた時点で直してよい。ユーザー承認は不要。

## エントリポイント

| 起点 | 場所 |
|---|---|
| CLI 引数・プロファイル解決・`--help`/`--version` | `main.go`（`resolveDSN` / `parseSaveProfile` / `selectProfile`） |
| DSN → アダプタ生成 | `internal/db/opener/opener.go` の `Open`（循環依存回避のための接着剤パッケージ） |
| Bubble Tea の `Init`/`Update`/`View` | `internal/ui/model.go` |
| クエリ実行の唯一の経路 | `internal/ui/query.go` の `prepareAndExecuteQuery` → `executeQueryCmd` → `adapter.Query` |

`adapter.Query` が唯一の実行経路であることは設計上の要（AI 生成 SQL・スニペット・履歴・サイドバー挿入もすべてここを通る）。
横断的な制約（readonly guard など）はここ 1 箇所に掛ければ全経路に効く。

## internal/db — データベース抽象層

| ファイル | 責務 |
|---|---|
| `adapter.go` | `DBAdapter` インターフェース（`Type` / `Query` / `Tables` / `Columns` / `Schema` / `QuoteIdentifier` / `Close`）と `QueryResult`・`Kind` |
| `open.go` | DSN のユーティリティ（`MaskDSN` / `DetectType` / `DisplayName` / `Placeholder` / `InitialQuery`） |
| `opener/` | DSN から各アダプタを生成する接着剤 |
| `dbutil/` | 全アダプタ共通。値の文字列化と kind 判定（`classifyValue` / `StringifyValueKind`）、行スキャン（`ScanRowsOpts`、10,000 行上限）、SQL スキャナ（`LeadingKeyword` / `CteBodyKeyword` / `ContainsReturning`） |
| `sqlite/`, `mysql/`, `postgres/` | 各 DB のアダプタ実装 |
| `bring/` | 持ち寄り先のローカル SQLite。`Open` / `Materialize`（affinity 決定 + 型付き bind）/ `recordProvenance`（`_asql_bring`） |

`QueryResult.Rows [][]string` は不変で、`Kinds [][]db.Kind` が意味（NULL/空文字/整数/浮動小数/バイナリ/テキスト）を並走して運ぶ。
`Kinds` が nil なら従来の全 TEXT 挙動にフォールバックする。

方言差の扱い: バイナリ判定は SQLite が Go の型、MySQL が列型と**判定材料が違う**ため、
共通化せず `ScanOptions.BytesAreBinary` でアダプタ側に選ばせている（詳細は LESSONS.md）。

## internal/ui — TUI 層

モードは 10 種（`model.go` の `mode` 定数）: NORMAL / INSERT / SIDEBAR / AI / EXPORT / DETAIL / SNIPPET / SEARCH / PROFILE / STATS。

| ファイル | 責務 |
|---|---|
| `model.go` | `model` 構造体、モード定数、`Update` のディスパッチ |
| `states.go` | 各オーバーレイモードの状態構造体（`detailState` / `aiState` / `snippetState` / `profileState` / `histSearchState` / `completionState` …） |
| `normal.go` / `insert.go` / `sidebar.go` | 主要 3 モードのキーハンドリング |
| `ai.go` / `export.go` / `detail.go` / `snippet.go` / `profile.go` / `history_search.go` / `stats.go` | 各オーバーレイモード |
| `query.go` | クエリ実行コマンド（上記エントリポイント） |
| `result.go` | 結果の反映・列幅計算・ソート・ビューポート同期 |
| `compare.go` | 横並び比較と差分ハイライト |
| `connmgr.go` | 複数接続の保持と切替（`Switch` / `Register` / `CloseAll`）。bring DB は `Register` 経由で `opener.Open` を通らない |
| `bring.go` | `b` で持ち寄り、`J` でローカル bring DB へ切替 |
| `stats.go` / `sparkline.go` / `histogram.go` | Stats overlay（`d` キー）とその描画部品 |
| `statusbar.go` | ステータスバー（`statusHints` / `statusPositionInfo` / `statusConnectionLabel`） |
| `overlay.go` | モーダルの幅計算と背景への重ね合わせ |
| `sanitize.go` / `cursor.go` / `sort.go` / `completion.go` | 表示サニタイズ、カーソル、ソート、Tab 補完 |
| `table/` | `bubbles/table` v1.0.0 の **vendor fork**（MIT、LICENSE 同梱）。ANSI 幅バグの 2 行パッチのみ適用 |

`table/` を fork している理由: 上流の修正は v2 系にしか入らず、v1 向け PR は未マージのまま閉じられた。
v2 移行は bubbletea/lipgloss v2 を巻き込むため見合わない。詳細は HISTORY.md。

## その他のパッケージ

| パッケージ | 責務 |
|---|---|
| `internal/export/` | CSV / JSON / Markdown への変換 |
| `internal/ai/` | OpenAI 互換 API のクライアント。スキーマをプロンプトに注入 |
| `internal/config/` | `~/.config/asql/config.yaml`（AI 設定はネスト形式 `ai:` 配下） |
| `internal/profile/` | 接続プロファイル（`profiles.yaml`） |
| `internal/snippet/` | クエリスニペットの永続化 |
| `internal/fsutil/` | `atomicWrite` 等のファイル操作ヘルパー |

## 設計文書

| 文書 | 内容 |
|---|---|
| `docs/readonly-design.md` | readonly mode の設計（未実装）。二層構成、SQL guard の分類ルール、実測結果、未決定事項 |
| `docs/*.tape`, `docs/setup-demo-db.py` | README 用の VHS デモ録画 |
| `e2e/` | VHS による E2E テスト（`run.sh` / `*.tape` / `setup-profiles.py`）。**実行前に `e2e/README.md` を必ず読む** |
| `status/` | 品質監査スキルの出力先（`review.md` 等） |
