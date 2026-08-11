# asql 開発ロードマップ (PLAN)

VISION.md に基づく今後の実装計画。
完了したタスクの履歴は `HISTORY.md` を参照。

## 方針

asql の次の感動ポイントは「比較観察の完成度」にある。
高機能化ではなく、芯を sharpen する時期。

1. **単体観察が気持ちいい** — Phase 1 完了。達成済み。
2. **比較観察が驚くほど軽い** — Phase 2 残タスクで完成させる。**最優先**。
3. **気づきに必要な情報だけが静かに見える** — Phase 4 軽量 insight で補強。比較観察の後に着手。

Bring & Join (Phase 3) はまだ先。比較体験が磨き込まれてから。

## 現状

- Phase 0 Infrastructure 完了
- Phase 1 Core Observation UX 全完了 (P0 + P1)
- Phase 2 Multi-DB: 複数接続同時保持 (2-1)、同一クエリ別DB実行 (2-2)、横並び表示 (2-3) 完了
- CLI: `--help` / `--version`、README 整備済み (v0.6.0)
- テストカバレッジ拡充 (Issue #14, PR #35): MySQL/PostgreSQL アダプタ + UI (insert/sidebar/profile) テスト追加完了
- Phase 4 完了: 4-1/4-2/4-3 Column Statistics Overlay (PR #36)、4-4 Sparkline (PR #38)、4-5 Histogram (PR #40)
- コード品質改善 (PR #39): バグ修正・重複解消・パフォーマンス防御・設計改善
- セキュリティ・安定性: Go toolchain pin to 1.26.2 (PR #42)、狭ターミナル応答性 (PR #43)、TUI レイアウト・モード遷移の堅牢化 (PR #44)
- 最新リリース: v0.10.0
- Phase 3 (Bring & Join) 着手: 3-1/3-2 完了 (`b`/`J` キー)、3-3 は要件未確定のため継続検討
- 定期メンテナンス: 直接依存 (`pgx v5.10.0`, `mysql v1.10.0`, `modernc.org/sqlite v1.53.0`) 更新、`govulncheck` 到達脆弱性 (GO-2026-5004) 解消
- Bring & Join の型情報保持と provenance 記録 (`_asql_bring`) 完了、README を実装に同期
- readonly mode は設計のみ完了（`docs/readonly-design.md`）。実装は未着手
- **次: readonly mode の実装可否判断、または Phase 3 残タスク (3-3)**

## 直近完了: Bring & Join の意味保存 — 型情報保持と provenance

目的:
- 持ち寄ったデータが「何だったか」を失わないこと。VISION の Bring Data Philosophy は、持ち寄った先で観察できて初めて成立する

主要ステップ:
- [x] `db.QueryResult` に `Kinds [][]db.Kind` を追加し、`dbutil.ScanRowsLimit` がスキャン時点でセルごとの意味（NULL / 空文字 / 整数 / 浮動小数 / バイナリ / テキスト）を記録
  > `Rows [][]string` は不変。描画・ソート・エクスポート・比較・詳細表示はすべて無改修で、NULL と空文字の表示仕様も維持される。`Kinds` が nil の `QueryResult` は従来の全TEXT挙動にフォールバックするため、既存の構造体リテラルは全て互換
- [x] `bring.Materialize` が Kinds から列ごとの SQLite affinity を決め、各セルを型付きで bind
  > 混在列は「型を宣言しない」（BLOB affinity）を選択。TEXT 宣言だと bind した int64 が text に潰れ、型を運んだ意味がなくなることを実測で確認した。affinity 判定は「実際に bind される値」（`effectiveKind`）で行い、int64 に収まらない値が INTEGER 列に入って REAL へ暗黙変換される事故を防ぐ
- [x] `bring.Source` と `_asql_bring` テーブルで持ち寄り元を記録（データと同一トランザクション）
  > 新しいモードもオーバーレイも追加せず、既存のクエリ経路で観察できる普通のテーブルにした。先頭アンダースコアでサイドバー先頭に並ぶ
- [x] Stats overlay の NULL 率を kind ベースに変更
- [x] README.md / README.ja.md を実装に同期（AI 設定の階層混在バグ、Stats overlay、Bring & Join、`d` キー）

結果:
- `go build ./... && go vet ./... && go test ./...` 全パス
- 追加テスト: affinity 決定、混在列の storage class 保持、数値ソート/JOIN、文字列 `"NULL"` と SQL NULL の区別、BLOB の hex 往復、int64 範囲外のフォールバック、provenance の記録とロールバック

## 未着手: readonly mode

目的:
- 本番DBを観察用途で安全に開く。守るのは「意図しない書き込み」であって「意図的な書き込み」ではない

設計は `docs/readonly-design.md` に記載済み。実装前に以下の決定が必要:
- [ ] セッション全体 (`--readonly`) か、プロファイル単位 (`readonly: true`) か、両方か
  > セッション全体だけなら `connManager` にフラグ1つで済み、配線が半分になる
- [ ] `profiles.yaml` にキーを増やすか（上と連動）
- [ ] 環境変数 `ASQL_READONLY` を用意するか

検証済みの前提:
- SQLite は `file:<path>?mode=ro` が有効（`PRAGMA query_only=0` でも解除できない）。一方 `_pragma=query_only(1)` は解除できてしまうので単独では信頼できない。`mode=ro` でも `ATTACH` は通るため、SQL guard 側での `attach` 拒否は必須
- MySQL の go-sql-driver は DSN の未知パラメータを新規コネクションごとに `SET` するため、プール越しにも効く（実サーバ未検証）
- AI 生成 SQL は textarea に挿入されるだけで実行は必ず `adapter.Query` を通るので、アダプタラッパ1箇所で自動的に同じ制約がかかる

## Phase 2: Multi-DB Observation — 比較の完成（完了）

目的：**「観察を加速する」**。本番と検証、異種DB間の「差」を浮き彫りにする。

- [x] 2-1. 複数接続同時保持
- [x] 2-2. 同一クエリを別DBで実行 (`R` 再実行 / `x` 接続切替+即実行)
- [x] 2-3. 横並び表示 (2つの結果セットを画面分割で並べて比較)
- [x] 2-4. 差分ハイライト (件数差・値の違いに即座に気づかせる)

## Phase 4: Light Insight Helpers（完了）

目的：**「軽さを損なわない範囲で、気づきを増やす」**。
比較観察と相性が良い。Phase 2 完了後に着手。

- [x] 4-1. NULL率表示 (PR #36: `d` キーで Stats overlay)
- [x] 4-2. distinct数表示 (PR #36: 同上)
- [x] 4-3. min/max表示 (PR #36: 同上)
- [x] 4-4. 件数推移の簡易表示 (PR #38: Stats overlay でカーソル行にスパークライン表示)
- [x] 4-5. 簡易ヒストグラム表示 (PR #40: Stats overlay で数値列に Unicode ブロック文字のヒストグラム表示)

## Phase 3: Bring & Join（3-1/3-2/3-4 完了、3-3 検討中）

目的：**「Bring Data Philosophy」**の体現。異種DBを直接統合せず、ローカルに持ち寄って気づく。

- [x] 3-1. クエリ結果をローカル一時テーブルに保存 (SQLite)
  > `b` キー。`internal/db/bring/` パッケージ、`internal/ui/bring.go`
- [x] 3-2. ローカルでのJOIN実行
  > `J` キーでローカルbring DBへ切替後、既存のクエリ実行パイプラインでJOINを実行
- [x] 3-4. 持ち寄り時の型情報保持と provenance 記録
  > `db.Kind` によるスキャン時点の意味記録と `_asql_bring` テーブル。詳細は HISTORY.md
- [ ] 3-3. 日次などの粒度統一サポート
  > 要件が曖昧なため先送り。着手前に「どのカラムをどう丸めるか」のUXを再検討する
