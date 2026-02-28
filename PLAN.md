# サンプルデータ用 SQL スクリプト追加

## 目的
`test.db` / `example.db` が空ファイルのため、sqly の動作確認時にテーブルやデータがない。手動テスト用にサンプルデータを用意する。

## タスク
- [x] `testdata/sample.sql` を作成（CREATE TABLE + INSERT で 3 テーブル分のサンプルデータ）
- [x] `example.db` と `test.db`（空の 0 バイトファイル）を削除

## 検証
- `go build` が通ること ✅
- `go test ./...` が通ること ✅
- `sample.sql` の SQL を Go + modernc.org/sqlite で実行し、JOIN クエリでデータ取得を確認 ✅
