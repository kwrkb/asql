# PLAN.md — フェーズ境界と現在地

正典は `VISION.md`。完了の記録は `HISTORY.md`、判断の記録は `LESSONS.md`。
個別の機能要望・バグは **GitHub Issue** で管理する。このファイルに手順分解は書かない。

## 方針

VISION が掲げた観察体験は Phase 1〜4 で出揃った。

1. **単体観察が気持ちいい** — Phase 1 完了。
2. **比較観察が驚くほど軽い** — Phase 2 完了（横並び + 差分ハイライト）。
3. **気づきに必要な情報だけが静かに見える** — Phase 4 完了（Stats overlay / sparkline / histogram）。
4. **持ち寄って観察する** — Phase 3 完了（3-1/3-2/3-4）。3-3（粒度統一）は 2026-08-11 に廃止。

最後の機能タスクだった readonly mode は実装済み。ここから **メンテナンス期**
（依存更新・脆弱性追従・バグ修正・実利用で見つかった摩擦の解消）に入り、以降の作業は Issue から拾う。
VISION.md の `Phase` を参照。

## 現在地

- Phase 0〜4 完了。readonly mode (`--readonly`) 実装済み。最新リリース **v0.11.0**
- **未着手の機能タスクなし。** 新しい作業は GitHub Issue から
- 定常作業: 依存更新と `govulncheck ./...` の到達脆弱性 0 件確認。
  CI が PR・main への push・毎週のスケジュール実行で `govulncheck` を回すので、検知は自動。リリース前のローカル確認は継続
- CI の解析 toolchain は `go.mod` の `go` ディレクティブ。**stdlib の脆弱性を消すのは
  ここを上げること**であって、ローカルの Go を上げることではない（ローカルが新しいと
  govulncheck はクリーンに見えるが、`go.mod` が古い patch を指したままなら CI は赤）

## 持ち越しの負債

メンテナンス期に維持し続けるもの。増やさないための一覧であって、着手予定リストではない。

- `internal/ui/table/` の vendor fork — bubbles v1 に上流修正が来ないため恒久。
  解消には bubbles/bubbletea/lipgloss v2 移行が必要で、2 行のパッチには見合わない
- readonly の層2（接続レベル）は **SQLite のみ検証済み**。MySQL / PostgreSQL は層1（文ガード）だけが防御。
  実サーバで検証できる機会があれば層2を足す。README に「readonly 接続だから安全」とは書かない

## フェーズ履歴

| フェーズ | 状態 |
|---|---|
| Phase 0. Infrastructure | 完了 |
| Phase 1. Core Observation UX (P0 + P1) | 完了 |
| Phase 2. Multi-DB Observation (2-1〜2-4) | 完了 |
| Phase 3. Bring & Join (3-1 / 3-2 / 3-4) | 完了。**3-3 粒度統一は廃止** |
| Phase 4. Light Insight Helpers (4-1〜4-5) | 完了 |

各フェーズの実装内容と PR 番号は `HISTORY.md` を参照。
