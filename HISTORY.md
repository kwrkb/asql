# asql 実装履歴 (Project History)

これまでに完了した主要な機能・マイルストーンの記録。

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
