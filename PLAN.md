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
- Phase 4 着手: 4-1/4-2/4-3 Column Statistics Overlay (PR #36)、4-4 Sparkline (PR #38)
- コード品質改善 (PR #39): バグ修正・重複解消・パフォーマンス防御・設計改善
- **次: Phase 4-5 (ヒストグラム) または Phase 3**

## 直近完了: コード品質・パフォーマンス改善 (PR #39)

目的:
- Codex 静的分析で特定されたバグ・重複・パフォーマンス問題を修正する

主要ステップ:
- [x] A1: stats.go `computeColumnStats` 境界チェック追加
- [x] A2: sparkline.go `truncateTime`/`bucketKey` の panic をフォールバックに変更
- [x] A3: AI エラーレスポンスの情報露出制限（構造化エラー抽出 + 200文字制限）
- [x] B1: DB 接続生成を `internal/db/opener` パッケージに一本化
- [x] B2: `containsReturning` を `dbutil.ContainsReturning` に共通化（~170行削減）
- [x] C1: `ScanRows` に10,000行上限追加 + Stats 計算を `tea.Cmd` で非同期化
- [x] D1: `config.Load()` の stderr 直出力を `Warnings` フィールドに変更

結果:
- 18ファイル変更、+299/-247行
- 全14パッケージのテスト・`go vet` パス

## Phase 2: Multi-DB Observation — 比較の完成（最優先）

目的：**「観察を加速する」**。本番と検証、異種DB間の「差」を浮き彫りにする。

- [x] 2-1. 複数接続同時保持
- [x] 2-2. 同一クエリを別DBで実行 (`R` 再実行 / `x` 接続切替+即実行)
- [x] 2-3. 横並び表示 (2つの結果セットを画面分割で並べて比較)
- [x] 2-4. 差分ハイライト (件数差・値の違いに即座に気づかせる)

## Phase 4: Light Insight Helpers（次の候補）

目的：**「軽さを損なわない範囲で、気づきを増やす」**。
比較観察と相性が良い。Phase 2 完了後に着手。

- [x] 4-1. NULL率表示 (PR #36: `d` キーで Stats overlay)
- [x] 4-2. distinct数表示 (PR #36: 同上)
- [x] 4-3. min/max表示 (PR #36: 同上)
- [x] 4-4. 件数推移の簡易表示 (PR #38: Stats overlay でカーソル行にスパークライン表示)
- [ ] 4-5. 簡易ヒストグラム表示 (将来)

## Phase 3: Bring & Join（後回し）

目的：**「Bring Data Philosophy」**の体現。異種DBを直接統合せず、ローカルに持ち寄って気づく。
前提：Phase 2 の比較体験が十分に磨き込まれてから。

- [ ] 3-1. クエリ結果をローカル一時テーブルに保存 (SQLite等)
- [ ] 3-2. ローカルでのJOIN実行
- [ ] 3-3. 日次などの粒度統一サポート
