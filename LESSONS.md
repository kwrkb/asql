# LESSONS.md

このプロジェクトで学んだパターン・教訓を記録する。同じミスを繰り返さないために参照する。

---

## SQL パーサ設計

### 先頭キーワードだけでは不十分なケースがある

**文脈**: `returnsRows()` が先頭キーワードのみで判定していたため、`INSERT ... RETURNING` が結果セットを返さなかった。

**学び**: SQL 文の「先頭キーワード」判定は一次フィルタに過ぎない。DML に `RETURNING` 句が付く場合など、句レベルの検出が必要になる。

**パターン**:
- 先頭キーワードで早期 return できるケース（SELECT/PRAGMA/WITH/EXPLAIN/VALUES）は先に処理
- それ以外は本文スキャン（`containsReturning()` 等）を実行
- スキャナは文字列リテラル・識別子・コメントをスキップする単語境界チェック必須

### SQL スキャナの実装チェックリスト

キーワード検出スキャナを書く際に必ず対処すること:

- [ ] `'...'` 単引用符リテラル（`''` エスケープ対応）
- [ ] `"..."` 二重引用符識別子（`""` エスケープ対応）
- [ ] `` `...` `` バッククォート識別子（SQLite/MySQL 方言）
- [ ] `[...]` ブラケット識別子（SQLite/MSSQL 方言）
- [ ] `--` 行コメント（改行まで）
- [ ] `/* ... */` ブロックコメント
- [ ] 単語境界チェック（前後が識別子文字でないこと）— 部分一致を防ぐ

**失敗例**: `"..."` の中で `""` エスケープを未処理にすると、`"a ""returning"" b"` のような識別子内のキーワードを誤検出する。バッククォート・ブラケットも同様に、スキップなしだと内側のキーワードが単語境界チェックをすり抜ける。

---

## バイナリデータの表示

### `[]byte` を無条件に `string()` 変換してはいけない

**文脈**: `stringifyValue()` が `[]byte` を `string(v)` で変換していたため、非UTF-8 BLOB が文字化けしていた。

**学び**: Go の `string([]byte)` は UTF-8 検証をしない。TUI や画面出力に使う場合は必ず validity チェックが必要。

**パターン**:
```go
case []byte:
    if utf8.Valid(v) {
        return string(v)
    }
    return fmt.Sprintf("%x", v) // hex 表示
```

---

## テスト設計

### 統合テストに含めるべきエッジケース（SQLite アダプタ）

- RETURNING 付き INSERT/UPDATE/DELETE
- BLOB カラム（hex 表示確認）
- NULL 値（`"NULL"` 文字列になること）
- 空文字・空白のみクエリ（エラーになること）
- 文字列リテラル内に SQL キーワードが含まれるケース（false positive 防止）
- `""` エスケープ識別子、バッククォート、ブラケット内のキーワード（false positive 防止）

### sentinel 行はカラム数に合わせてパディングすること

**文脈**: `applyResult()` で「(no rows)」sentinel を `table.Row{"(no rows)"}` で作っていた。カラム数が 2 以上のとき Row の長さが足りずパニックの原因になりうる。

**パターン**:
```go
sentinel := make(table.Row, len(columns))
sentinel[0] = "(no rows)"
rows = []table.Row{sentinel}
```

---

## コードレビュー指摘への対応

### Gemini / Codex bot レビューの扱い方

`/gemini-audit` や Codex bot の指摘は、人間レビュアーがいない場合でも実際のバグを含む場合がある。`/resolve-pr-comments` スキルで分類し、妥当な指摘は対応する。

**修正の優先順位**:
1. High — 機能バグ（結果セットが返らない等）→ 最優先
2. Medium — 保守性・安全性 → High の直後に対処
3. Low — スタイル・最適化 → 余裕があれば

**bot 指摘の判断基準**: コードをトレースして実際に false positive / false negative が発生するか確認してから対応を決める。「bot だから無視」はしない。

---

## VHS (GIF 録画)

### VHS v0.10.0 の Width/Height はピクセル値

**文脈**: `Set Width 120` `Set Height 35` と指定したら `Dimensions must be at least 120 x 120` や ffmpeg の pad エラーが発生した。

**学び**: VHS v0.10.0 では `Set Width` / `Set Height` はピクセル単位。ドキュメントのデフォルト値（80/24）は古いバージョンの文字単位の名残。最低 120x120 ピクセルが必要。

**パターン**:
```
Set Width 1200
Set Height 600
Set FontSize 16
```

### Hide ブロック後に clear が必要

**文脈**: `Hide` / `Show` でセットアップコマンドを隠したが、Show 後の最初のフレームにセットアップコマンドの出力が残っていた。

**学び**: `Hide` は VHS のフレームキャプチャを停止するだけで、ターミナルの表示状態はリセットしない。Show 前に `clear` を入れてターミナルをクリーンにする。

**パターン**:
```
Hide
Set TypingSpeed 1ms
Type "setup-command"
Enter
Sleep 500ms
Type "clear"
Enter
Sleep 200ms
Show
```

---

## TUI 設計

### リストUIにはスクロールオフセットが必須

**文脈**: サイドバーのテーブル一覧が常にインデックス 0 から描画されていたため、テーブル数が表示可能行数を超えるとカーソルが画面外に出てしまうバグ（Gemini bot が検出）。

**学び**: カーソル付きリスト UI を実装する際、表示範囲外にカーソルが出るケースを必ず考慮する。描画開始位置（スクロールオフセット）をカーソル位置に追従させる。

**パターン**:
```go
maxVisible := height - headerLines
scrollOffset := 0
if m.cursor >= maxVisible {
    scrollOffset = m.cursor - maxVisible + 1
}
for i := scrollOffset; i < len(items); i++ { ... }
```

### スクロール計算では空行・セパレータを含む全描画行数を数える

**文脈**: Detail View の `linesPerField = 2`（label + value）としていたが、実際にはフィールド間にセパレータ用の改行があり3行消費していた。結果 `maxVisibleFields` が過大評価され、短いターミナルで選択フィールドがモーダル外にはみ出るバグ（Claude + Codex の Consensus で検出）。

**学び**: スクロール計算の「1アイテムあたりの行数」は、装飾・空行・セパレータを含めた **実際の描画行数** を数えること。コードコメントの「label + value」のような概念的な記述と実装の乖離に注意。

**パターン**: 描画ループ内で `WriteByte('\n')` を何回呼んでいるか数えて `linesPerField` を決める。

### 新しい描画パスには既存の sanitize() を忘れずに適用する

**文脈**: テーブル描画では `sanitize()` を適用していたが、新規追加した Detail View オーバーレイでは colName / colType / val を未サニタイズで描画していた。Gemini bot のレビューで検出。同様に、プロファイル名やステータスバーのテキストもサニタイズ漏れがあった。

**学び**: 同じデータを別の UI コンポーネントで描画する場合、既存パスで適用済みのサニタイズ処理を新パスでも漏れなく適用すること。特に TUI では ANSI エスケープシーケンスによる UI スプーフィングリスクがある。`setStatus` や `fmt.Sprintf` に外部由来の文字列（プロファイル名、DB名等）を渡す際も sanitize する。

### ソートで NULL を「常に末尾」にするには比較関数の外で処理する

**文脈**: `smartCompare` で NULL に `+1`（末尾）を返していたが、DESC ソート時に `cmp = -cmp` で符号反転され、NULL が先頭に来てしまった。

**学び**: 「NULL は方向に関係なく常に末尾」のようなソート不変条件は、比較関数の返り値を反転する前に独立して処理する必要がある。比較関数内の NULL ハンドリングだけでは DESC 反転で壊れる。

**パターン**:
```go
sort.SliceStable(indices, func(i, j int) bool {
    // NULL は方向に関係なく常に末尾 — 比較反転の外で処理
    if aNULL != bNULL {
        return bNULL // bがNULLならaが前
    }
    cmp := smartCompare(a, b)
    if dir == sortDesc {
        cmp = -cmp
    }
    return cmp < 0
})
```

### 表示ロジックの重複は早期に統合する

**文脈**: `applyResult` と `applyResultWithSort` で列ヘッダ構築・行変換・sentinel 処理が完全に重複していた。自己レビューで検出し、`applyResult` を `applyResultWithSort` への委譲に統合。同様に `Ctrl+S` のスニペット保存ロジックも `normal.go` / `insert.go` で重複していたため、`enterSnippetNamingMode()` ヘルパーに統合。クエリ実行ロジック（cancel/history/execute）も4箇所で重複→ `prepareAndExecuteQuery` に統合。

**学び**: 「モード名の違い」「インジケータの有無」程度の差分で関数を複製すると、片方の修正がもう片方に反映されないバグの温床になる。差分が小さい場合は `m.mode` 等の現在の状態を参照するヘルパーに一本化する。重複に気づくのはレビュー時が多いため、新しいコードパスを追加する際に「既存で同じことをしている箇所はないか」を先に探す。

### if/else チェーンでフォーカス判定と状態チェックを混ぜない

**文脈**: ステータスバーのカーソル位置表示で `if m.pinned != nil && m.comparePane == 0 && len(Rows) > 0` としていた。pinned ペインにフォーカスがあるが0行の場合、条件全体が false になり `else if` に落ちてアクティブペイン（非フォーカス側）の情報が表示されるバグ。

**学び**: 「どのペインにフォーカスがあるか」と「そのペインにデータがあるか」は別の判定。外側の if でフォーカスを確定し、内側で状態（行数等）をチェックする。混ぜると else に意図しないフォールスルーが起きる。

**パターン**:
```go
if m.pinned != nil && m.comparePane == 0 {
    // フォーカスは pinned — ここで確定
    if len(p.result.Rows) > 0 {
        posInfo = ...
    }
    // 0行なら posInfo は空のまま（アクティブペインの情報は出さない）
} else if len(m.lastResult.Rows) > 0 {
    posInfo = ...
}
```

### サイズ制限付きキャッシュの Evict 戦略は全クリアを避ける

**文脈**: `completionColCache` が上限 64 に達したとき `make(map[...])` で全エントリを破棄していた。テーブル数が上限前後のDBでキャッシュスラッシングが起き、毎回 `Columns()` クエリが走る問題（Gemini レビューで検出）。

**学び**: 外部ライブラリなしで簡易キャッシュを実装する場合、全クリアではなくランダム1件削除（Go の `for range map` + `break`）で十分。LRU ほどの精度は不要でも、ホットエントリの大半を保持できる。

**パターン**:
```go
if len(cache) >= maxSize {
    for k := range cache {
        delete(cache, k)
        break // 1件だけ削除
    }
}
```

### import 削除はファイル内の全参照を確認してから行う

**文脈**: `Ctrl+S` ロジックを `snippet.go` のヘルパーに抽出した際、`normal.go` から `strings` と `textinput` の import を削除した。しかし `textinput.Blink` が AI モード（`Ctrl+K`）でも使われており、ビルドエラーになった。

**学び**: コードの一部を別ファイルに移動した際、移動元ファイルから import を削除する前に、同じ import を使う他の箇所がファイル内に残っていないか確認する。Go コンパイラが即座にエラーを出すので致命的ではないが、確認を怠ると手戻りになる。

---

### connManager 等のリソース管理は終了時の CloseAll を保証する

**文脈**: 初期アダプタは `defer adapter.Close()` で閉じていたが、TUI 内でプロファイル切替により開いた追加接続は `connMgr.CloseAll()` が呼ばれずリーク。コードレビューで検出。

**学び**: リソースマネージャ（接続プール等）を導入した場合、個別リソースの Close ではなくマネージャの CloseAll を `defer` する。個別 Close との二重解放にも注意。

**パターン**: model に `CloseAll()` メソッドを公開し、`main.go` で `defer m.CloseAll()` する。初期アダプタの個別 `defer adapter.Close()` は削除。

### nil チェック vs 境界チェック — メソッドレシーバの nil は実質到達不能

**文脈**: `connManager` の `Active()` 等で `cm == nil` チェックがあったが、mutex ロック取得が先なので nil なら先にパニックする。実質到達不能なのに安心感のための nil チェックが残っていた。

**学び**: ポインタレシーバのメソッドで `cm == nil` チェックを書くより、到達可能な実際のバグ（`cm.active >= len(cm.conns)` 等の境界違反）をガードする方が有用。

---

## データエクスポート

### map キーによる JSON 変換は重複カラム名でデータ損失する

**文脈**: `FormatJSON` で `map[string]string` にカラム名をキーとして格納していた。`SELECT a.id, b.id FROM ...` のように同名カラムがあると、後の値が前の値を上書きし、片方のデータがサイレントに失われた。

**学び**: SQL クエリ結果のカラム名は一意とは限らない。`map` のキーに使う場合は重複を検出してサフィックスを付与する必要がある。CSV/Markdown は配列ベースなので影響なし。

**パターン**: 2パス方式 — 先に全出現回数を数え、重複があれば全出現に `_1`, `_2` を付与する（最初の出現だけサフィックスなしだと混乱を招く）:
```go
func deduplicateHeaders(headers []string) []string {
    total := make(map[string]int, len(headers))
    for _, h := range headers { total[h]++ }
    seen := make(map[string]int, len(headers))
    result := make([]string, len(headers))
    for i, h := range headers {
        seen[h]++
        if total[h] > 1 {
            result[i] = fmt.Sprintf("%s_%d", h, seen[h])
        } else {
            result[i] = h
        }
    }
    return result
}
```

---

## LLM 統合（AI 機能）

### http.Client にはデフォルトタイムアウトを設定する

**文脈**: AI クライアントの `http.Client{}` にタイムアウトを設定していなかった。呼び出し側で context タイムアウトがあっても、クライアント自体に防御がないとコンテキストなしで呼ばれた場合にハングする。

**学び**: 外部 API と通信する `http.Client` は必ずデフォルトタイムアウトを持たせる。コンテキストのタイムアウトとは別レイヤーの防御。

**パターン**:
```go
httpClient: &http.Client{Timeout: 30 * time.Second},
```

### AI 生成コンテンツには実行前警告を出す

**文脈**: LLM が生成した SQL をエディタに挿入する際、ステータスメッセージが「SQL generated by AI」だけで警告が弱かった。Prompt injection による破壊的 SQL のリスク。

**学び**: human-in-the-loop であっても、AI 生成コンテンツには明示的な「レビューしてから実行せよ」の警告を出すべき。特に SQL は破壊的操作が可能。

**パターン**: ステータスに `"AI generated SQL — review before executing"` のように行動指示を含める。

### システムプロンプトにユーザーデータを埋め込む際はコードフェンスで区切る

**文脈**: DB スキーマをエスケープなしで system prompt に埋め込んでいた。悪意あるテーブル名・カラム名（例: `"; DROP TABLE users; --`）でプロンプトインジェクションが可能。

**学び**: LLM のプロンプトに外部データを注入する場合、データ部分をコードフェンス（` ``` `）やXML タグで明確に区切る。完全な防御ではないが、LLM がデータと指示の境界を認識しやすくなる。

### 非同期操作のキャンセルには stale msg 対策が必須

**文脈**: `context.WithCancel` で操作をキャンセル可能にしたが、キャンセル後に即再実行すると、古いリクエストの遅延 msg が新しい操作の `queryCancel` を `nil` クリアしてしまい、新しい操作がキャンセル不能になった。Codex レビューで検出。

**学び**: Bubble Tea の非同期 Cmd は完了順序が保証されない。キャンセル→再実行のフローでは、古い msg が新しい状態を壊す可能性がある。操作ごとにシーケンス番号を振り、msg ハンドラで照合して stale msg を破棄する。

**パターン**:
```go
// model に seq カウンタを持つ
type model struct {
    querySeq    uint64
    queryCancel context.CancelFunc
}

// 操作開始時にインクリメント
m.querySeq++
ctx, cancel := context.WithCancel(context.Background())
m.queryCancel = cancel
return m, executeQueryCmd(ctx, m.db, query, m.querySeq)

// msg に seq を含める
type queryExecutedMsg struct {
    seq    uint64
    result db.QueryResult
    err    error
}

// ハンドラで照合
case queryExecutedMsg:
    if msg.seq != m.querySeq {
        return m, nil // stale msg → 無視
    }
```

### 新規リクエスト開始時に既存 context を明示的にキャンセルする

**文脈**: 実行中の操作がある状態で新しいリクエストを発行すると、古い context の `CancelFunc` が上書きされ、古い goroutine がタイムアウトまでリソースを消費し続けた。Gemini レビューで検出。

**学び**: `CancelFunc` を上書きする前に必ず既存のものを呼び出す。seq による stale msg 破棄だけでは不十分で、リソースリーク防止には明示的キャンセルが必要。

**パターン**:
```go
if m.queryCancel != nil {
    m.queryCancel()
}
ctx, cancel := context.WithCancel(context.Background())
m.querySeq++
m.queryCancel = cancel
```

### macOS では os.UserConfigDir() が XDG_CONFIG_HOME を無視する

**文脈**: `Load()` で `os.UserConfigDir()` を使っていたが、テストで `t.Setenv("XDG_CONFIG_HOME", tmpDir)` しても macOS では反映されず、テストが失敗した。WSL (Linux) では `os.UserConfigDir()` が `$XDG_CONFIG_HOME` を参照するため通っていた。

**学び**: Go の `os.UserConfigDir()` は OS ごとに挙動が異なる:
- **Linux**: `$XDG_CONFIG_HOME` → fallback `~/.config`
- **macOS**: **常に** `~/Library/Application Support`（`XDG_CONFIG_HOME` を無視）
- **Windows**: `%AppData%`

テストで設定ディレクトリを差し替えたい場合、`os.UserConfigDir()` だけに依存すると macOS で壊れる。

**パターン**: `XDG_CONFIG_HOME` を明示的に先にチェックするヘルパーを挟む:
```go
func configDir() (string, error) {
    if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
        return d, nil
    }
    return os.UserConfigDir()
}
```

### 秘密情報を含む設定ファイルは環境変数オーバーライドとパーミッションチェックを入れる

**文脈**: `config.yaml` に AI API キーが平文保存されていた。環境変数からの読み取り手段がなく、CI やコンテナ環境で不便。

**学び**: API キー等の秘密情報は (1) 環境変数を最優先で読む (`ASQL_AI_API_KEY` 等)、(2) ファイルのパーミッションが 0600 より緩い場合は警告を出す、の2層で防御する。

**注意**: 環境変数オーバーライド処理は「設定ファイル不在」の早期リターンより**後**に配置しがち。ファイルが無くても環境変数だけで動作する必要がある場合、早期リターンせずにゼロ値 Config に対して環境変数を適用すること（Codex レビューで検出）。

### os.UserConfigDir() 等の環境エラーを握りつぶさない

**文脈**: `os.UserConfigDir()` がエラーを返した場合に `Config{}, nil` を返していた。設定ディレクトリのパーミッションエラー等がサイレントに無視され、デバッグ困難に。

**学び**: 「ファイルが存在しない」と「環境が壊れている」は区別すべき。前者はゼロ値で正常、後者はエラーとして返す。

**パターン**:
```go
dir, err := os.UserConfigDir()
if err != nil {
    return Config{}, fmt.Errorf("finding user config dir: %w", err)
}
// ファイル不在は os.IsNotExist で判定して nil error を返す
```

---
