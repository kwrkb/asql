# Ctrl+C による操作キャンセル機能

## 目的
Ctrl+C で実行中の操作（クエリ/AI生成）をキャンセルし、アプリは終了せず normalMode に戻る挙動を実装する。

## タスク
- [x] 1. `model` 構造体に `queryCancel context.CancelFunc` フィールド追加
- [x] 2. `executeQueryCmd` を変更: 外部から context を受け取る形に
- [x] 3. `generateSQLCmd` を変更: 同様に外部 context 受取り
- [x] 4. `updateInsert` の `ctrl+enter`/`ctrl+j` ハンドラで cancelable context を生成して model に保存
- [x] 5. `updateAI` の `enter` ハンドラで同様に cancelable context を生成
- [x] 6. `Update()` の `tea.KeyMsg` 分岐冒頭で `ctrl+c` をインターセプト
- [x] 7. `updateAI` の `aiLoading` ガードで `esc` を通す
- [x] 8. `queryExecutedMsg` / `aiResponseMsg` ハンドラで `queryCancel = nil` クリア + context.Canceled 処理
- [x] 9. ステータスバーのヒントに `C-c:cancel` 追加（クエリ/AI実行中の表示）
- [x] 10. ドキュメント更新（README.md, README.ja.md, CLAUDE.md）

## 検証
- `go build` 成功 ✅
- `go test ./...` 全パス ✅
- `go vet ./...` 成功 ✅

## 変更履歴
- (なし)
