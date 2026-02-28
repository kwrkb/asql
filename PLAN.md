# Multi-DB 対応 (MySQL / PostgreSQL)

## 目的
asql を SQLite 専用から MySQL/PostgreSQL 対応に拡張する。

## タスク

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

## 検証
- `go test ./...` 全パス ✓
- `go vet ./...` クリーン ✓
- `go build` 成功 ✓

## 変更履歴
- Phase 0-2: 全フェーズ一括実装完了
