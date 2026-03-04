# asql 実装履歴 (Project History)

これまでに完了した主要な機能・マイルストーンの記録。

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
