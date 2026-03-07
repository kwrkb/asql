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

## よくあるハマりポイント

### Width/Height はピクセル値

`Set Width 120` のように文字数のつもりで書くと `Dimensions must be at least 120 x 120` エラーになる。VHS v0.10+ ではピクセル単位。

```
# OK
Set Width 1200
Set Height 600
Set FontSize 16
```

### Wait 構文は `Wait+Screen /<regex>/`

旧ドキュメントの `Wait 5s "INSERT"` は動かない。正しくは `Wait+Screen /<regex>/`。タイムアウトは `Set WaitTimeout` で設定。

```
Set WaitTimeout 10s
Wait+Screen /INSERT/
Wait+Screen /alice@example\.com/   # 正規表現なので . はエスケープ
```

### オーバーレイで隠れたテキストは検出できない

`Wait+Screen` は画面に実際に表示されている文字列のみマッチする。例えば Export オーバーレイ表示時にステータスバーの `EXPORT` は隠れているので検出できない。オーバーレイ内のテキスト（`Export Results` 等）を使う。

### Hide/Show はバッファをクリアしない

`Hide` は VHS のフレームキャプチャを停止するだけ。Hidden フェーズのコマンド出力はバッファに残る。セットアップコマンドは `>/dev/null 2>&1` で出力を抑制すること。

### asql は INSERT モードで起動する

`Type "i"` で INSERT に入ろうとすると、文字 `i` がエディタに入力されてしまう。起動直後は既に INSERT モードなので、`Ctrl+l` でクリアしてからクエリを入力する。

### compare モードには十分な幅が必要

`Set Width 1200` / `Set FontSize 16` だと compare モードで「Terminal too narrow」エラーになる場合がある。split ビューを使う tape では `Set Width 1800` / `Set FontSize 14` 程度にする。

### TUI 起動コマンドを Hidden フェーズで使わない

`./asql --save-profile ...` のように TUI が起動するコマンドを Hidden フェーズで実行すると、TUI が入力待ちでハングする。プロファイル等の事前設定は VHS 外（`run.sh` のセットアップ等）で行う。
