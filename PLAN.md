# UI/UX改善: テーブルサイドバー & キーバインドヒント

## 目的
テーブル一覧をサイドバーで表示し、選択して即クエリ挿入できるようにする。ステータスバーにキーバインドヒントを追加する。

## タスク

### Phase 1: DBAdapter 拡張（完了）
- [x] `internal/db/adapter.go`: `Tables(context.Context) ([]string, error)` を追加
- [x] `internal/db/sqlite/adapter.go`: SQLite 用 `Tables()` 実装
  > `sqlite_master` から `type='table'` を `ORDER BY name` で取得
- [x] `internal/db/sqlite/adapter_test.go`: `Tables()` のテスト
  > 空DB、複数テーブルソート順、VIEW除外の3ケース

### Phase 2: サイドバー UI（完了）
- [x] model にサイドバー状態追加 + sidebarMode
- [x] `tablesLoadedMsg` / `loadTablesCmd()` / Init() でロード
- [x] `renderSidebar()` カスタム描画
- [x] `View()` 修正: sidebar 開時は横並びレイアウト
- [x] `resize()` 修正: sidebar 幅考慮
  > 幅60未満でサイドバー自動閉じ対応
- [x] `updateSidebar()`: j/k/Enter/Esc キー処理
  > Enter でテーブル名を正しくクォートし `SELECT * FROM "table" LIMIT 100;` を挿入
- [x] NORMAL モードに `t` キー追加
- [x] クエリ実行後にテーブル一覧リロード

### Phase 3: ステータスバー改善（完了）
- [x] モード別キーバインドヒント表示
  > NORMAL: `t:tables i:insert q:quit` / INSERT: `C-Enter:exec Esc:normal` / SIDEBAR: `j/k:nav Enter:select Esc:close`

### Phase 4: README デモ GIF & レビュー対応（完了）
- [x] VHS tape ファイル作成 (`docs/demo.tape`, `docs/setup-demo-db.py`)
- [x] `vhs docs/demo.tape` で `docs/demo.gif` 生成（181KB, 340フレーム）
- [x] README.md / README.ja.md: Screenshot→Demo、サイドバーキーバインド追加
- [x] `docs/screenshot.svg` 削除
- [x] サイドバースクロール修正（Gemini レビュー指摘対応）

## 検証
- `go test ./...` 全テスト通過
- `go build` 成功
- PR #6 squash マージ完了
