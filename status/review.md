# asql 全体コードレビュー

**レビュー日**: 2026-03-22
**対象**: コードベース全体 (Phase 2 完了時点)

---

## 1. 総合評価

**品質スコア: 8/10** — プロダクション品質。アーキテクチャは明確で、テストカバレッジも良好。

### 強み

- **DBAdapter パターン**: 3 アダプタ (SQLite/MySQL/PostgreSQL) が一貫したインターフェースを実装。`adapter.go` の 7 メソッドが明確で過不足ない
- **Bubble Tea の正しい活用**: immutable update、`querySeq` による stale 結果排除、`context.CancelFunc` によるクエリ中断
- **SQL パーサーの堅牢性**: CTE、RETURNING、コメント、各種クォート (double/backtick/bracket/dollar-quote) のスキップ処理がエッジケースまでテスト済み
- **connManager のスレッドセーフティ**: `sync.RWMutex` で保護、Bubble Tea ループ外からのアクセスに対応
- **LESSONS.md による知見蓄積**: 24KB の実践的ノウハウ。同じミスの繰り返しを防止
- **CI/CD**: GitHub Actions (test + vet) + GoReleaser でクロスプラットフォームリリース自動化

---

## 2. 要修正 (Critical)

### 2-1. スニペット/プロファイルの非アトミック書き込み

| 項目 | 詳細 |
|------|------|
| **場所** | `internal/snippet/snippet.go:71` — `os.WriteFile(path, data, 0o644)` |
|  | `internal/profile/profile.go:71` — `os.WriteFile(path, data, 0o600)` |
| **問題** | `Save()` がファイルを直接上書き。書き込み途中でクラッシュするとファイル破損 |
| **影響** | スニペットやプロファイル設定の喪失 |
| **修正案** | 一時ファイルに書き込み → `os.Rename()` でアトミックに置換 |

### 2-2. Completion の同期 DB アクセスが UI をブロック

| 項目 | 詳細 |
|------|------|
| **場所** | `internal/ui/completion.go:282-284` — `getOrFetchColumns()` |
| **問題** | Tab 補完時にカラム情報を同期的に DB から取得 (1s タイムアウト)。Bubble Tea の Update ループ内で実行されるため、最大 1 秒間 UI がフリーズする |
| **影響** | ネットワーク遅延のある MySQL/PostgreSQL で顕著。ユーザー体験の劣化 |
| **修正案** | 非同期 Cmd でカラムを取得し、結果を Msg 経由で反映。キャッシュヒット時のみ同期応答 |

---

## 3. 改善推奨 (Medium)

### 3-1. Open() で PingContext 未使用

| 項目 | 詳細 |
|------|------|
| **場所** | `internal/db/sqlite/adapter.go:25`, `mysql/adapter.go` 同様, `postgres/adapter.go:27` |
| **問題** | `conn.Ping()` が context なしで呼ばれる。ネットワーク問題時に無期限ブロックの可能性 |
| **修正案** | `conn.PingContext(ctx)` に変更し、5 秒程度のタイムアウト付き context を渡す |

### 3-2. Connection Pool 未設定

| 項目 | 詳細 |
|------|------|
| **場所** | 全アダプタの `Open()` 関数 |
| **問題** | `SetMaxOpenConns()`, `SetMaxIdleConns()`, `SetConnMaxLifetime()` を設定していない |
| **影響** | 長時間動作する TUI でコネクションリークの可能性 (ドライバデフォルトに依存) |
| **修正案** | TUI 用途に適した保守的な設定 (例: MaxOpen=5, MaxIdle=2, Lifetime=5m) |

### 3-3. PostgreSQL buildCreateTable での手動クォート

| 項目 | 詳細 |
|------|------|
| **場所** | `internal/db/postgres/adapter.go:121` — `fmt.Sprintf("  %s %s", name, dataType)` |
|  | `internal/db/postgres/adapter.go:134` — 手動で `"` + ReplaceAll |
| **問題** | カラム名に `QuoteIdentifier()` を使用していない。テーブル名は手動クォート |
| **影響** | 予約語や特殊文字を含むカラム名で不正な DDL が生成される可能性 |
| **修正案** | `a.QuoteIdentifier()` を使用して統一 |

### 3-4. Completion カラムキャッシュのランダム eviction

| 項目 | 詳細 |
|------|------|
| **場所** | `internal/ui/completion.go:290-295` |
| **問題** | 64 テーブル上限に達すると `for k := range` で最初に見つかったキーを削除。Go の map イテレーションはランダム順のため、頻繁に使うテーブルのキャッシュも削除される |
| **修正案** | LRU キャッシュに変更。または `container/list` + map で実装 |

### 3-5. スニペットファイルのパーミッション不一致

| 項目 | 詳細 |
|------|------|
| **場所** | `internal/snippet/snippet.go:71` — `0o644` |
|  | `internal/profile/profile.go:71` — `0o600` |
| **問題** | プロファイルは DSN (パスワード含む) を保存するため `0o600` だが、スニペットは `0o644`。スニペットに機密クエリが含まれる場合のリスク |
| **修正案** | スニペットも `0o600` に統一を検討 |

### 3-6. tablesLoadedMsg のエラー非通知

| 項目 | 詳細 |
|------|------|
| **場所** | `internal/ui/model.go:371-378` |
| **問題** | `msg.err != nil` の場合、テーブルリストが更新されないだけでユーザーに通知されない |
| **修正案** | `m.setStatus("Failed to load tables: "+msg.err.Error(), true)` でステータスバーに表示 |

---

## 4. 軽微な指摘 (Minor)

### 4-1. sanitize() の Fast-path 不足

- **場所**: `internal/ui/` 各レンダリング箇所
- **問題**: ANSI コードが含まれないセルにも毎回 `sanitize()` を適用
- **修正案**: ASCII 範囲チェックで早期リターン

### 4-2. DSN マスキングの範囲

- **場所**: `internal/db/open.go:29` — `q.Get("password")` のみ
- **問題**: `api_key`, `secret` 等のクエリパラメータは未マスク
- **影響**: 現在の用途では低リスク。必要に応じて拡張

### 4-3. Compare モードの positional diff

- **場所**: `internal/ui/compare.go`
- **問題**: 行インデックスベースの比較のため、行順序が異なると false positive が発生
- **方針**: 意図的な設計であれば LESSONS.md に明記を推奨

### 4-4. snippet/profile パッケージの configDir() 重複

- **場所**: `internal/snippet/snippet.go:16-21` と `internal/profile/profile.go:16-21`
- **問題**: 同一の `configDir()` 関数が 2 箇所に重複定義
- **修正案**: `internal/config/` パッケージに共通化。ただし現状のコード量では過剰かもしれない

---

## 5. アーキテクチャ観察

### Model struct の規模

`internal/ui/model.go` の model struct は約 50 フィールド + 5 つのサブ状態 struct を持つ。現時点では管理可能だが、Phase 4 (Insight Helpers) 追加時にはサブモデル分割を検討すべき。

### モード管理

9 つのモード (`NORMAL/INSERT/SIDEBAR/AI/EXPORT/DETAIL/SNIPPET/PROFILE/SEARCH`) は `mode` 文字列で管理。暗黙的な遷移ルールは各ファイルに分散している。明示的な FSM (有限状態機械) の導入は過剰だが、許可される遷移をコメントで文書化すると保守性が向上する。

### SQL パーサーの設計

`dbutil.LeadingKeyword()` → `CteBodyKeyword()` → `containsReturning()` の 3 段階アプローチは堅牢。各アダプタに `containsReturning()` が重複しているのは意図的 (SQLite: bracket-quote対応、PostgreSQL: dollar-quote対応) で、適切な設計判断。

---

## 6. テストカバレッジ

### 充実している領域

| 領域 | ファイル | 特記事項 |
|------|---------|---------|
| SQL パーサー | `sqlite/adapter_test.go`, `postgres/adapter_test.go` | CTE、RETURNING、コメント、クォートのエッジケース網羅 |
| DB ユーティリティ | `dbutil/dbutil_test.go` | `StringifyValue` (NULL/blob/time)、`LeadingKeyword`、`ShortenTypeName` |
| Completion | `completion_test.go` | `wordAtCursor`、`detectContext`、`lastKeyword` |
| Sort | `sort_test.go` | 数値/文字列/NULL の混合ソート |
| AI クライアント | `ai/client_test.go` | モック HTTP サーバーで正常/エラーパス |
| connManager | `connmgr_test.go` | 311 行。スレッドセーフティ、接続切替 |
| E2E | `e2e/*.tape` (5 本) | 起動、クエリ実行、モード遷移、エクスポート、エラー表示 |

### ギャップ

| 領域 | 現状 | 推奨 |
|------|------|------|
| モード遷移 Update() | ユニットテストなし | 主要遷移パスのテスト追加 |
| Compare モード | E2E テープなし | VHS テープ追加 |
| ベンチマーク | なし | 大規模結果セット (1000+ 行) のレンダリング性能計測 |
| MySQL/PostgreSQL 統合 | 環境変数依存 (CI 未実行) | CI に Docker サービスコンテナ追加を検討 |
| Export clipboard | E2E 未テスト | VHS + clipboard コマンドで検証 |

---

## 7. 推奨アクション (優先度順)

1. **snippet/profile の atomic write** — データ喪失防止 (工数: 小)
2. **getOrFetchColumns の非同期化** — UI フリーズ防止 (工数: 中)
3. **PingContext 導入** — 接続タイムアウト保証 (工数: 小)
4. **tablesLoadedMsg エラー通知** — ユーザビリティ向上 (工数: 小)
5. **PostgreSQL buildCreateTable の QuoteIdentifier 統一** — 正確性 (工数: 小)
6. **Connection Pool 設定** — リソース管理 (工数: 小)
7. **Completion キャッシュ LRU 化** — 補完品質 (工数: 中)
