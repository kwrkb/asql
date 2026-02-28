# sqly → asql リネーム

## 目的
プロジェクト名・バイナリ名・モジュールパス・設定パスをすべて `sqly` から `asql` に変更する。

## タスク

### Phase 1: Go コード変更（コア）
- [x] `go.mod` — module パスを `github.com/kwrkb/asql` に変更
- [x] `main.go` — import パス 5箇所 + エラーメッセージ「sqly exited」→「asql exited」
- [x] `internal/db/sqlite/adapter.go` — import パス 1箇所
- [x] `internal/ui/model.go` — import パス 2箇所
- [x] `internal/ui/export.go` — import パス 1箇所
- [x] `internal/ui/model_test.go` — import パス 1箇所
- [x] `internal/config/config.go` — 設定ディレクトリ `"sqly"` → `"asql"`
- [x] `internal/config/config_test.go` — テスト内ディレクトリ名 `"sqly"` → `"asql"`

### Phase 2: ドキュメント・設定ファイル
- [x] `README.md` — プロジェクト名、URL、コマンド例、設定パス
- [x] `README.ja.md` — 同上（日本語版）
- [x] `CLAUDE.md` — プロジェクト説明、コマンド例、設定パス
- [x] `.gitignore` — バイナリ名 `sqly` → `asql`

### Phase 3: デモ・テストデータ
- [x] `docs/demo.tape` — タイトル、DB パス名、コマンド
- [x] `docs/setup-demo-db.py` — DB パス名
- [x] `testdata/sample.sql` — コメント内の参照 + プロダクト名

### Phase 4: クリーンアップ
- [x] `status/quality-report.md` — パス参照の更新
- [x] 旧バイナリ `sqly` を削除
- [x] `go build` でビルド確認
- [x] `go test ./...` で全テスト通過確認
- [x] `grep -r "sqly"` でリネーム漏れなし確認

## 検証
- `go build` 成功 ✓
- `go test ./...` 全パス ✓
- `grep -r "sqly" --include="*.go" --include="*.md" --include="*.mod"` で漏れなし ✓
