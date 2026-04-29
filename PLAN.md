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
- **次: Phase 3 (Bring & Join)**

## 直近完了: TUI レイアウト・モード遷移の堅牢化 (PR #44)

目的:
- Codex レビューで検出した狭ターミナルでのレイアウト崩れとモード遷移時の状態残留を解消する

主要ステップ:
- [x] `width - N` / `height - N` のオフセット計算に `<= 0` ガードと `max()` クランプを徹底
- [x] AI/Snippet/Profile/HistorySearch の Blur を `blurActiveInput()` ヘルパーに集約
- [x] モーダル overlay の `textinput.Width` を `resize()` で `calcModalWidth` から動的同期（`View()` の純粋性を回復）
- [x] `renderCompareView` 等が View() 経路で行っていた状態変異を `Update`/`resize` に移動
- [x] 新たな運用ルールを LESSONS.md に追記（View 純粋化 / 幅ガード / モーダル入力幅同期 / モード別 Blur ヘルパー）

結果:
- 17ファイル変更、+120/-48行
- 全パッケージのテスト・`go vet` パス
- 横 ~30 列、縦 ~10 行の狭画面で全モーダル（AI/Snippet/Profile/Export/History）が破綻せず描画可能

過去の「直近完了」は `HISTORY.md` を参照（コード品質・パフォーマンス改善 PR #39 等）。

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

## Phase 3: Bring & Join（後回し）

目的：**「Bring Data Philosophy」**の体現。異種DBを直接統合せず、ローカルに持ち寄って気づく。
前提：Phase 2 の比較体験が十分に磨き込まれてから。

- [ ] 3-1. クエリ結果をローカル一時テーブルに保存 (SQLite等)
- [ ] 3-2. ローカルでのJOIN実行
- [ ] 3-3. 日次などの粒度統一サポート
