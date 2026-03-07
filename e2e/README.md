# E2E Tests

VHS を使った TUI の E2E テスト。テキストアサーション + MP4 録画による目視確認。

## 前提

- [VHS](https://github.com/charmbracelet/vhs) (v0.10+)
- [ttyd](https://github.com/tsl0922/ttyd) / [ffmpeg](https://ffmpeg.org/) — VHS が内部で使用

```bash
# macOS
brew install vhs ttyd ffmpeg

# Go
go install github.com/charmbracelet/vhs@latest
```

## テスト実行

```bash
bash e2e/run.sh
```

全 tape を順に実行し、PASS/FAIL を表示する。MP4 録画は `e2e/recordings/` に出力される。

## 目視確認の手順

1. テストを実行する

   ```bash
   bash e2e/run.sh
   ```

2. 録画を再生する

   ```bash
   # まとめて開く
   open e2e/recordings/*.mp4

   # 個別に確認
   open e2e/recordings/01_startup.mp4
   ```

3. 確認ポイント
   - テーブルの罫線やカラムが崩れていないか
   - モード表示（INSERT / NORMAL / SIDEBAR）が正しい位置にあるか
   - オーバーレイ（Export、Detail View）のレイアウトが正常か
   - エラーメッセージがステータスバーに表示されているか

## tape 一覧

| tape | 内容 |
|------|------|
| `01_startup.tape` | 起動、INSERT→NORMAL 遷移、サイドバー、テーブル一覧 |
| `02_query_exec.tape` | クエリ入力・実行、結果表示 |
| `03_mode_transitions.tape` | INSERT→NORMAL→SIDEBAR→NORMAL→INSERT のモード遷移 |
| `04_export.tape` | クエリ実行後の Export オーバーレイ表示 |
| `05_error.tape` | 不正 SQL のエラーメッセージ表示 |

## 録画について

- 各 tape の `Output` 行で `e2e/recordings/<name>.mp4` に出力
- `e2e/recordings/` は `.gitignore` 済み（ローカル専用）
- `TypingSpeed 50ms` + 操作間 `Sleep` で目視しやすい速度に調整済み
