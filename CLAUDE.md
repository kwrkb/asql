# CLAUDE.md

This file provides guidance to AI assistants when working with code in this repository.

## Project Overview

**asql = Data Observation CLI**
asql は Go 製の軽量 TUI SQL クライアント。データを「速く・見やすく・並べて触れる」ことで違和感や仮説に気づくための「データ観察ツール（顕微鏡）」であり、巨大な分析基盤ではない。
- **Module**: `github.com/kwrkb/asql`
- **Framework**: Bubble Tea (Charmbracelet)
- **Support**: SQLite, MySQL, PostgreSQL
- **AI**: OpenAI 互換 API による Text-to-SQL 補助機能

## Commands

```bash
# ビルド
go build

# 実行
./asql <sqlite-file-path>
./asql "mysql://user:pass@host:3306/dbname"
./asql "postgres://user:pass@host:5432/dbname"

# テスト
go test ./...

# E2E テスト（VHS）— 実行前に e2e/README.md を必ず読むこと
bash e2e/run.sh

# 静的解析
go vet ./...

# リリース（ローカル実行）
# 1. PLAN.md / HISTORY.md を更新してコミット & push（タグはコミットを指すので必須）
# 2. git status クリーン確認 + go vet ./... + go test ./... + govulncheck ./...
#    （govulncheck は CI でも回るが、タグを打つコミットで到達脆弱性 0 件を最終確認する）
# 3. goreleaser check で .goreleaser.yml の deprecation を事前検証
git tag v<version>
git push origin v<version>
GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
```

## Architecture

ファイル単位の詳しい索引（どのモードがどのファイルにあるか、主要関数の場所）は **`NOTES.md`** を参照。

- **internal/db/** — データベース抽象層。
  - `adapter.go` — `DBAdapter` インターフェース（`Type`, `Query`, `Tables`, `Columns`, `Schema`, `QuoteIdentifier`, `Close`）。
  - `dbutil/` — 全アダプタ共通のユーティリティ（`returnsRows` 判定、値の文字列化など）。
  - `opener/` — DSN から各アダプタを生成する接着剤パッケージ（循環依存回避）。
  - `sqlite/`, `mysql/`, `postgres/` — 各 DB のアダプタ実装。
- **internal/ui/** — TUI 層。`model.go` (Bubble Tea) を中心に責務別ファイル分割 (`normal.go`/`insert.go`/`sidebar.go`/`detail.go`/`compare.go`/`overlay.go`/`sanitize.go` 等)。モードは NORMAL/INSERT/SIDEBAR/AI/EXPORT/DETAIL/SNIPPET/SEARCH/PROFILE/STATS の 10 種。
- **internal/export/** — CSV/JSON/Markdown フォーマット変換ロジック。
- **internal/ai/** — LLM クライアント。スキーマ情報をプロンプトに注入。
- **internal/config/** — `~/.config/asql/config.yaml` の管理。
- **internal/profile/**, **internal/snippet/** — 接続プロファイルとクエリスニペットの永続化。
- **internal/fsutil/** — `atomicWrite` 等のファイル操作共通ヘルパー。
- **internal/ui/table/** — `bubbles/table` v1.0.0 の vendor fork（ANSI 幅バグの 2 行パッチ）。上流 v1 に修正が来ないため恒久。

## Design Principles (from VISION.md)

1. **軽さを最優先**: 起動・反応速度を損なわない。
2. **思考を止めない**: キーボード中心。SQL を書き直さずに探索できる UX。
3. **ノイズを排除**: UI は目立たず、情報は必要なものだけ。
4. **観察を加速**: 比較や結合、基礎統計へのアクセスを容易にする。
5. **Bring Data Strategy**: 異種 DB を直接統合せず、ローカルに持ち寄って比較・結合する。

## Workflow Files

**優先順位（矛盾したとき、上が勝つ）: `VISION.md` > `PLAN.md` > コード > `NOTES.md`**
`HISTORY.md` / `LESSONS.md` は**序列外**（仕様ではなく記録）。

**コードが `VISION.md` と矛盾していたら、勝手にコードへ寄せず作業を止めて確認する。**

| ファイル | 役割 | 更新のルール |
|---|---|---|
| `VISION.md` | **正典**。目的・スコープ・非スコープ・Decision Rule・現在のフェーズ | **更新はユーザー承認必須** |
| `PLAN.md` | フェーズ境界と現在地**だけ** | 手順分解は書かない（TodoWrite を使う）。個別の機能要望・バグは **GitHub Issue** |
| `HISTORY.md` | 完了したタスクの永続記録 | PLAN.md から落ちた完了項目はここへ移す |
| `LESSONS.md` | 判断の記録 | **追記専用**。過去を書き換えない。1 エントリ = 却下した案 / 決め手（観測事実のみ）/ 覆す条件。冒頭の「書き方」を参照 |
| `NOTES.md` | 内部構造の索引 | **揮発・鮮度非保証**。コードが常に正しい。気づいた時点で直してよい（承認不要） |
| `docs/` | 設計文書（例: `readonly-design.md`） | 実装前のレビュー対象 |
| `status/` | 品質監査スキルの出力先（`review.md` 等） | — |

現在のフェーズは **メンテナンス期**（VISION.md の `Phase`）。
新機能は Decision Rule を満たすだけでは足りず、「実利用で不足が観察された」ことを条件とする。
