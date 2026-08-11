# PLAN.md — フェーズ境界と現在地

正典は `VISION.md`。完了の記録は `HISTORY.md`、判断の記録は `LESSONS.md`。
個別の機能要望・バグは **GitHub Issue** で管理する。このファイルに手順分解は書かない。

## 方針

VISION が掲げた観察体験は Phase 1〜4 で出揃った。

1. **単体観察が気持ちいい** — Phase 1 完了。
2. **比較観察が驚くほど軽い** — Phase 2 完了（横並び + 差分ハイライト）。
3. **気づきに必要な情報だけが静かに見える** — Phase 4 完了（Stats overlay / sparkline / histogram）。
4. **持ち寄って観察する** — Phase 3 完了（3-1/3-2/3-4）。3-3（粒度統一）は 2026-08-11 に廃止。

残る未着手の機能は readonly mode ただ 1 つ。これを入れた後は **メンテナンス期**
（依存更新・脆弱性追従・バグ修正・実利用で見つかった摩擦の解消）に移り、以降の作業は Issue から拾う。
VISION.md の `Phase` を参照。

## 現在地

- Phase 0〜4 完了。最新リリース **v0.10.0**
- readonly mode は設計のみ完了（`docs/readonly-design.md`）。実装未着手 — **これが最後の機能タスク**
- 持ち越しの負債: `internal/ui/table/` の vendor fork（bubbles v1 に上流修正が来ないため恒久。v2 移行は未定）
- 定常作業: 依存更新と `govulncheck ./...` の到達脆弱性 0 件確認

## 残タスク: readonly mode

目的: 本番 DB を観察用途で安全に開く。守るのは「意図しない書き込み」であって「意図的な書き込み」ではない。

- [ ] SQL guard（層1）と SQLite `mode=ro`（層2）を実装する
  > 設計・分類ルール・実測結果は `docs/readonly-design.md`。着手時のスコープは
  > **セッション全体 `--readonly` のみ**（`profiles.yaml` のキーと `ASQL_READONLY` は足さない）
- [ ] 層2 を MySQL / PostgreSQL の実サーバで検証する
  > 確認が取れなければ層1のみで出荷してよい。ただし README に「readonly 接続だから安全」とは書かない

## フェーズ履歴

| フェーズ | 状態 |
|---|---|
| Phase 0. Infrastructure | 完了 |
| Phase 1. Core Observation UX (P0 + P1) | 完了 |
| Phase 2. Multi-DB Observation (2-1〜2-4) | 完了 |
| Phase 3. Bring & Join (3-1 / 3-2 / 3-4) | 完了。**3-3 粒度統一は廃止** |
| Phase 4. Light Insight Helpers (4-1〜4-5) | 完了 |

各フェーズの実装内容と PR 番号は `HISTORY.md` を参照。
