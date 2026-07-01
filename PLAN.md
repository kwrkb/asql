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
- **次: Phase 3 残タスク (3-3) または Phase 2 横展開**

## 直近完了: Phase 3 着手 — Bring & Join (3-1/3-2)

目的:
- VISION.md の Bring Data Philosophy を実現する第一歩。異種DBから取得したクエリ結果をローカルSQLiteに持ち寄り、JOINで観察できるようにする

主要ステップ:
- [x] `internal/db/bring/` パッケージ新規作成。`Materialize()` が結果セットを全カラム TEXT 型で `CREATE TABLE` し、パラメータ化バッチINSERTで投入
  > `QueryResult.Rows` はスキャン時点で `[][]string` に文字列化済み（型情報喪失）のため、軽量実装として全TEXT保存・文字列一致JOINを採用。`"NULL"`/`""` の表示センチネルは INSERT 時に実NULL/空文字へ逆変換するが、本物の文字列値 `"NULL"` との曖昧さは既知の制約として受容
- [x] `internal/db/sqlite/adapter.go` に `NewAdapter(conn *sql.DB)` を追加し、bring パッケージが INSERT 用とJOIN実行用で同一コネクションプールを共有できるようにリファクタ
- [x] `internal/ui/connmgr.go` に `Register()` を追加。`opener.Open` を経由せず bring 用ローカルDBを `connManager` に直接登録（固定の合成DSN `asql-bring` を使用し、`Switch` 経由の意図しない二重DB生成を回避）
- [x] `internal/ui/bring.go` 新規作成。`b` キーでアクティブな結果をローカルテーブル (`t1`, `t2`, ...) として持ち寄り、`J` キーでローカルbring DBへ接続切替
  > 新しい `mode` やオーバーレイは追加せず、既存のクエリ実行パイプライン（INSERTモード→`executeQueryCmd`→`applyResult`）をそのまま再利用。Stats/Export/Sort/Compare がJOIN結果に対しても無改修で動作する設計判断

結果:
- `go build && go vet ./... && go test ./...` 全パス
- tmux 上での手動smoke test: 2つのSQLite DBから `users`/`scores` をそれぞれ `b` で持ち寄り、`J` で切替、`SELECT t1.name, t2.score FROM t1 JOIN t2 ON t1.id = t2.id` が正しい1行を返すことを確認。Stats overlay (`d`) もJOIN結果に対して動作

過去の「直近完了」は `HISTORY.md` を参照（TUI レイアウト堅牢化 PR #44、コード品質・パフォーマンス改善 PR #39 等）。

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

## Phase 3: Bring & Join（3-1/3-2 完了、3-3 検討中）

目的：**「Bring Data Philosophy」**の体現。異種DBを直接統合せず、ローカルに持ち寄って気づく。

- [x] 3-1. クエリ結果をローカル一時テーブルに保存 (SQLite)
  > `b` キー。`internal/db/bring/` パッケージ、`internal/ui/bring.go`
- [x] 3-2. ローカルでのJOIN実行
  > `J` キーでローカルbring DBへ切替後、既存のクエリ実行パイプラインでJOINを実行
- [ ] 3-3. 日次などの粒度統一サポート
  > 要件が曖昧なため先送り。着手前に「どのカラムをどう丸めるか」のUXを再検討する
