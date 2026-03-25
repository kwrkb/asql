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
- Phase 4 着手: 4-1/4-2/4-3 Column Statistics Overlay (PR #36)
- **次: Phase 4 残タスク (4-4, 4-5) または Phase 3**

## 作業中: v0.8.0 VHS / E2E 更新

目的:
- `v0.8.0` 時点の主要UXを VHS で再現し、E2E の自動確認範囲を compare / stats まで広げる

変更対象ファイル:
- `PLAN.md`
- `e2e/run.sh`
- `e2e/README.md`
- `e2e/*.tape`

主要ステップ:
- [x] 現行の VHS / e2e 実行基盤と `v0.8.0` UI 差分を確認する
- [x] compare / stats をカバーする tape を追加する
- [x] 実行スクリプトのセットアップと環境依存を整理する
- [x] `go test` と VHS 実行結果を確認する

確認方法:
- `GOCACHE=/tmp/asql-gocache go test ./...`
- `bash e2e/run.sh`

想定リスク（影響範囲）:
- terminal 幅や VHS 待機条件のズレで flaky になる
- profile セットアップ方法次第で compare tape がハングする

結果:
- `06_compare.tape` と `07_stats.tape` を追加し、`v0.8.0` の主要UXを VHS でカバー
- `e2e/setup-profiles.py` を追加し、compare 系 tape を self-contained にした
- `e2e/run.sh` に `GOCACHE` / `XDG_CONFIG_HOME` の分離と profile セットアップを追加
- `run.sh` の `set -e` 下での `((passed++))` 早期終了バグを修正
- 検証結果: `go test ./...` 成功、`bash e2e/run.sh` で 7 passed / 0 failed

未解決事項:
- なし

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
- [ ] 4-4. 件数推移の簡易表示 (スパークライン等)
- [ ] 4-5. 簡易ヒストグラム表示 (将来)

## Phase 3: Bring & Join（後回し）

目的：**「Bring Data Philosophy」**の体現。異種DBを直接統合せず、ローカルに持ち寄って気づく。
前提：Phase 2 の比較体験が十分に磨き込まれてから。

- [ ] 3-1. クエリ結果をローカル一時テーブルに保存 (SQLite等)
- [ ] 3-2. ローカルでのJOIN実行
- [ ] 3-3. 日次などの粒度統一サポート
