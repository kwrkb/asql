# asql 実装履歴 (Project History)

これまでに完了した主要な機能・マイルストーンの記録。

## Phase 3: Bring & Join — 型情報保持と provenance
**実装**: `QueryResult` に `Kinds [][]db.Kind`（NULL/Empty/Int/Float/Blob/Text）を追加し、スキャン時点でセルごとの意味を記録。`bring.Materialize` はこれを使って (a) 列ごとに SQLite affinity を宣言（INTEGER/REAL/TEXT/BLOB、混在列は型無し宣言＝BLOB affinity で各値の storage class を保持）、(b) 各セルを int64/float64/[]byte/string/NULL として bind する。数値ソート・JOINが文字列比較にならず、SQL NULL と文字列 `"NULL"`、空文字と `""` が区別可能になった。`Rows [][]string` は不変なので描画・ソート・エクスポート・比較の各経路は無改修、NULL/空文字の表示仕様も維持。`Kinds` が nil の `QueryResult` は従来の全TEXT挙動にフォールバックする。
併せて `bring.Source`（持ち寄り順・ローカル表名・取得元接続・元クエリ）を導入し、bring DB 内の `_asql_bring` テーブルへデータと同一トランザクションで記録。行数・列数・truncated フラグも保持する。専用モードやオーバーレイは追加せず、既存のクエリ経路で `SELECT * FROM _asql_bring` として観察できる。ステータスバーは `(local bring: N tables)` を表示。
Stats overlay の NULL 率も表示文字列一致ではなく kind で判定するようになり、値が文字列 `"NULL"` の列でも正確になった。
**PR #50 レビュー対応**: (1) 整数と浮動小数が混在する列の `REAL` 宣言を撤回。SQLite の REAL affinity は bind した整数を浮動小数へ強制変換するため `9007199254740993` が `9007199254740992` に壊れていた。型無し宣言なら各値の storage class が保たれ、`ORDER BY` の数値順と `integer 3 = real 3.0` の JOIN も成立する。(2) `[]byte` の UTF-8 妥当性ではなく driver 報告の列型でバイナリ判定するよう変更。中身がたまたま UTF-8 妥当な BLOB 列が TEXT として持ち寄られ `X'"'"'616263'"'"'` との比較が外れていた。go-sql-driver は charset で `TEXT`/`BLOB` を分けるため MySQL の文字列データには影響しない。(3) provenance のクエリを `queryHistory` 末尾ではなく `queryExecutedMsg.query` 経由で受理された結果に紐付け（履歴は実行前に追記される「試行の記録」のため、後続クエリの失敗時に誤ったクエリが記録されていた）。(4) bring DB に接続中の持ち寄りでステータスバーの件数が更新されない問題を修正。

---

## bubbles/table の ANSI 幅バグ修正（vendor + 2行パッチ）
**実装**: `bubbles/table` v1.0.0 は `runewidth.Truncate` でセル幅を測るが、`runewidth.StringWidth` は ANSI エスケープのバイトを表示幅として数える。列幅が約27セル未満だとスタイル付き文字列がエスケープの途中で切断され、端末が残りを飲み込むため、**カラム型注釈が不可視**になり、**比較モードの差分セルは内容ごと消える**状態だった（幅の広い列では正常に描画されるため長期間気づかれなかった）。
上流の修正 charmbracelet/bubbles#884（`ansi.Truncate` への2行変更）はマージ先が v2 系のみで、v1 向け同一 PR #883 は未マージのまま閉鎖。bubbles v2 移行は bubbletea v2 / lipgloss v2 を巻き込むため見合わないと判断し、`internal/ui/table/` に v1.0.0 の `table.go`（450行・MIT、LICENSE 同梱）を vendor して上流と同じ2行だけを適用した。`x/ansi` は既に間接依存のため新規依存はゼロ。上流のテストスイートは vendor せず、パッチ対象の挙動だけを覆う `table_test.go` を自前で用意（未修正コードで落ちることを確認済み）。型注釈・選択カラムの reverse 強調・差分セルのハイライトがすべて色付きのまま復活。

---

## ドキュメント同期
**実装**: README.md の AI 設定 Examples がフラット形式（`ai_endpoint:` をトップレベル）で書かれており、`internal/config` が期待するネスト形式（`ai:` 配下）と食い違っていた問題を修正。両 README に Stats overlay（`d` キー、NULL率・distinct・min/max・sparkline・histogram・10k行スキップ）と Bring & Join（`b`/`J` の手順、型保持、`_asql_bring` の列定義）の節を追加。キーバインド表に欠けていた `d` と STATS モードを追加。README.ja.md に AI の環境変数オーバーライド表を追加。

---

## 定期メンテナンス — 依存更新と到達脆弱性の解消
**PR**: #48, #49
**実装**: 直接依存を更新（`jackc/pgx/v5 v5.10.0`、`go-sql-driver/mysql v1.10.0`、`modernc.org/sqlite v1.53.0`）し、`govulncheck ./...` が検出した到達可能な脆弱性 GO-2026-5004（pgx のプレースホルダ混同による SQL インジェクション）を解消。`go vet` / `go test` が全パスでも `govulncheck` は独立に回す必要があることを確認した（LESSONS.md「定期メンテナンス」）。依存更新時は `go.mod` の `go`/`toolchain` ディレクティブが意図せず引き上げられていないか diff で確認する。

---

## TUI レイアウト・モード遷移の堅牢化
**PR**: #43, #44
**実装**: 極端に狭い端末での描画崩れとモード遷移の不整合を修正。`overlay.go` の `calcModalWidth` の下限を実画面幅でクランプし、モーダルが画面外へはみ出さないようにした。AI / Snippet / Profile / SearchHistory の各 overlay で `textinput.Width` が固定値のまま `modalWidth` の縮小に追従していなかった問題は、`View()` 内での状態変異ではなく `resize()` への集約で解決した（「`View()` は純粋関数として保つ」ルール。PR #44 のレビューで 4 箇所の違反を指摘され追加コミットで是正）。

---

## Go toolchain の pin
**PR**: #42（closes #41）
**実装**: `go.mod` の toolchain を `go1.26.2` に pin。以降の依存更新でこのディレクティブが意図せず引き上げられていないことを毎回確認する。

---

## CLI インターフェース整備 (v0.6.0)
**実装**: `--help` / `--version` を追加し、README.md / README.ja.md を整備。引数解析は `main.go` の `resolveDSN` / `parseSaveProfile`（手書きパーサ）。

---

## Phase 4: Histogram (4-5)
**PR**: #40
**実装**: Stats overlay (`d` キー) で数値列に Unicode ブロック文字のヒストグラム (▁▂▅█▇▃▁) を表示。等幅 binning（最大 20 bin）、`renderSparklineBars` を再利用。BIGINT UNSIGNED ラベルのオーバーフロー、混合型列（>50% パース失敗で抑制）、NaN/Inf 除外、10,000 行上限のガードを実装。`detectNumericColumn` は word-boundary マッチで INTERVAL/POINT 等の誤検出を回避。

---

## コード品質・パフォーマンス改善
**PR**: #39
**実装**: Codex 静的分析に基づく横断的改善。(A) バグ修正: stats 境界チェック、sparkline panic 除去、AI エラー情報露出制限。(B) 重複解消: DB 接続生成を `opener` パッケージに一本化、`containsReturning` を `dbutil.ContainsReturning` に共通化（~170行削減）。(C) パフォーマンス: ScanRows 10,000行上限、Stats 計算の非同期化。(D) 設計: config 警告を戻り値化。18ファイル、+299/-247行。

---

## Phase 4: Sparkline (4-4)
**PR**: #38
**実装**: Stats overlay でカーソル行の日付/タイムスタンプ列をスパークライン (▁▂▅█▇▃▁) として表示。日次/月次の bucketing、最大幅制限、`truncateTime`/`bucketKey` のフォールバック処理（panic 除去は PR #39）。

---

## Phase 4: Column Statistics Overlay (4-1/4-2/4-3)
**PR**: #36
**実装**: NORMALモードで `d` キーにより各カラムの NULL率・distinct数・min/max をオーバーレイ表示。`lastResult.Rows` からインメモリ計算（DBクエリ不要）。j/k ナビゲーション、スクロール対応。15テスト。

---

## テストカバレッジ拡充 (Issue #14)
**PR**: #35
**実装**: MySQL/PostgreSQL アダプタと UI（insert/sidebar/profile モード）のテストカバレッジ拡充。31テスト関数、+599行。DB アダプタは Type/QuoteIdentifier/Open エラーパスをカバー。UI テストは各モードのキーバインド・状態遷移・境界条件を網羅。レビュー指摘を受け、境界/履歴テストを `t.Run` サブテストにリファクタ。

---

## Phase 2: 差分ハイライト (2-4)
**PR**: #31
**実装**: 比較モードで行位置ベースの差分検出。ラベルに `rows:N`、ステータスバーに件数差サマリ表示。値不一致・片側のみの行/列をセル強調。sentinel は強調対象外。

---

## Phase 2: 複数接続同時保持 (2-1)
**PR**: #26
**実装**: `connManager` による複数DB接続の同時保持。プロファイル切替時に既存接続を再利用し、新規DSNのみ接続。`sync.RWMutex` でデータ競合防止。`MaskDSN` を `net/url.Parse` ベースに刷新（クエリパラメータのパスワードもマスク）。接続切替時の実行中クエリキャンセルと stale msg 無効化。`extractHost` も `url.Parse` ベースに統一。

---

## 多カラムテーブルの表示崩れ修正 (1-13)
**PR**: #24
**実装**: 水平カラムウィンドウイング。画面幅を超える列数のテーブルで、表示可能な列のみを動的に選択して描画。h/l キーでカラムカーソル移動、colOffset ベースで可視範囲を自動調整。ANSI エスケープシーケンス注入対策として sanitize 関数を適用。

## 複数DB接続基盤の構築 (SQLite, MySQL, PostgreSQL)
**期間**: 初期開発フェーズ
**目的**: asql を特定のDB専用ではなく、様々な環境で使える「軽量ハブ」にすること。

### Phase 0: リファクタ
- [x] 0-1. `DBAdapter` に `Type() string` を追加
- [x] 0-2. `internal/db/dbutil/` 共通ユーティリティ作成
- [x] 0-3. SQLite adapter を dbutil 利用にリファクタ
- [x] 0-4. AI プロンプトに DB 種別注入
- [x] 0-5. TUI 初期テキストを DB 種別で分岐
- [x] 0-6. Phase 0 検証

### Phase 1: MySQL 対応
- [x] 1-1. 依存追加
- [x] 1-2. MySQL adapter 実装
- [x] 1-3. main.go に DSN 自動判定追加
- [x] 1-4. ステータスバーに DB 種別表示
- [x] 1-5. ドキュメント更新
- [x] 1-6. Phase 1 検証

### Phase 2: PostgreSQL 対応
- [x] 2-1. 依存追加
- [x] 2-2. PostgreSQL adapter 実装
- [x] 2-3. main.go の PostgreSQL 分岐有効化
- [x] 2-4. ドキュメント更新
- [x] 2-5. Phase 2 検証

---

## Core Observation UX (Phase 1 初期)
**目的**: Data Observation CLI としての基本的な見やすさと操作性の確保。

- [x] 1-1. 列幅自動調整 (データの長さに合わせる。無駄な余白を排除)
- [x] 1-8. エクスポート機能 (CSV/JSON/Markdown クリップボード・ファイル保存)

---

## Phase 0: Infrastructure
**目的**: 品質ゲートとコードベースの健全性を確保し、以降の開発速度を上げる。

- [x] 0-1. CI: GitHub Actions にテスト自動実行 (`go test ./...` + `go vet ./...`)
- [x] 0-2. refactor: model.go のモード別分割 (normal/insert/sidebar/ai/export に分離)
- [x] 0-3. security: DSN セキュリティ (環境変数 `ASQL_DSN` / `DATABASE_URL` 対応、パスワードマスキング)

---

## Phase 1 P0: Core Observation UX
**目的**: データへの気づきを増やす基本的な見やすさと操作性。

- [x] 1-1. NULL / 空文字 / 0 の視覚的な区別
- [x] 1-2. 型情報表示 (ヘッダに `name text` 形式)
- [x] 1-3. ソート (h/l でカラム選択、s でトグル ASC/DESC/None)
- [x] 1-4. ページング位置表示 (`col:name 1/100` 形式)
- [x] 1-5. クエリ履歴 (セッション内、Ctrl+P/Ctrl+N ナビゲーション)

---

## Phase 1 P1: Core Observation UX (Should Have)
**目的**: データ観察の利便性をさらに向上。

- [x] 1-6. Detail View Mode (行詳細表示)
  > Enter でオーバーレイ表示。j/k でフィールド移動、n/N で行遷移、q/Esc/Enter で閉じる。sanitize() でANSIエスケープ対策済み。
- [x] 1-12. PgUp/PgDn キーによるテーブル高速スクロール
- [x] 型名短縮表示 (`ShortenTypeName`: INTEGER→int, TIMESTAMPTZ→tstz 等) + dim スタイル適用
- [x] ソート時の行表示バグ修正 (Detail View で `table.Rows()` を参照するよう変更)
- [x] 1-7. 保存クエリ (スニペット機能)
  > `~/.config/asql/snippets.yaml` に名前付きクエリを永続化。NORMAL モードで `S` でブラウズ、`Ctrl+S` で保存（INSERT モードからも可）。モーダル内で Enter:ロード、d:削除、a:追加。`internal/snippet/` パッケージで永続化層を分離。
- [x] 1-10. クエリ履歴のインクリメンタル検索
  > Ctrl+R でモーダル検索。インクリメンタルフィルタ、C-p/C-n/C-r でナビゲーション。
- [x] 1-11. TUIエディタ操作の洗練
  > Ctrl+L でエディタクリア。
- [x] 1-9. テーブル名・カラム名の入力補完
  > `DBAdapter.Columns()` 追加（SQLite/MySQL/PostgreSQL）。SQL 文脈判定で FROM→テーブル / SELECT→カラムを切替。`tablename.` ドットプレフィックス対応。候補1つで即確定、複数でスクロール付きポップアップ表示。
- [x] 1-8. 接続プロファイル管理の永続化
  > `~/.config/asql/profiles.yaml` に接続プロファイルを永続化（0600パーミッション）。CLI で `--save-profile <name>` で保存、`@name` で接続。引数なし時はプロファイル選択UI表示。TUI 内で `P` キーでプロファイル一覧モーダル（a:追加、d:削除、Enter:DSN表示）。`internal/profile/` パッケージで永続化層を分離。`maskDSN` を `MaskDSN` としてエクスポート。
