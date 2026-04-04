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

### Hide/Show はターミナルバッファをクリアしない

**文脈**: `Hide` / `Show` でセットアップコマンドを隠したが、Show 後の最初のフレームにセットアップコマンドの出力が残っていた。

**学び**: `Hide` は VHS のフレームキャプチャを停止するだけで、ターミナルの表示状態はリセットしない。Hidden フェーズのコマンド出力はそのままバッファに残る。対策として (1) コマンド出力を `>/dev/null 2>&1` で抑制、(2) TUI アプリが全画面を上書きする場合はそれに頼る。`clear` を Hidden フェーズ内で実行しても完全にはクリアされないケースがある。

### --save-profile 等の TUI 起動コマンドは VHS の Hidden フェーズで使えない

**文脈**: `./asql --save-profile prod db.db` を Hidden フェーズで実行したところ、プロファイル保存後に TUI が起動してハングし、後続コマンドが実行されなかった。

**学び**: CLI ツールが「設定保存 + TUI 起動」を一体で行う場合、VHS tape 内で非対話的に実行できない。プロファイル等の事前設定は VHS 外で行い、tape 内では DB セットアップスクリプトのみ実行する。

### compare モードには十分なターミナル列数が必要

**文脈**: `Set Width 1200` / `Set FontSize 16` で GIF を生成したところ、`c` キー押下時に「Terminal too narrow for compare」エラーが表示された。

**学び**: VHS のピクセル幅とフォントサイズからターミナル列数が決まる。asql の compare モードは `minWidthForCompare = 80` 列が必要。split ビューでは各ペインに十分な幅が要るため、`Set Width 1800` / `Set FontSize 14` 程度を使う。

### VHS v0.10.0 の Wait 構文は `Wait+Screen /<regex>/`

**文脈**: `Wait 5s "INSERT"` と書いたら `Invalid command` エラーが大量に出た。

**学び**: VHS v0.10.0 の Wait 構文は旧ドキュメントと異なる。正しくは `Wait+Screen@<timeout> /<regexp>/`。タイムアウトは `Set WaitTimeout` でグローバル設定可能。

**パターン**:
```
Set WaitTimeout 10s
Wait+Screen /INSERT/
Wait+Screen /alice@example\.com/
```

### オーバーレイが画面を覆うとステータスバーのテキストが Wait で検出できない

**文脈**: Export オーバーレイ表示時に `Wait+Screen /EXPORT/` が失敗。ステータスバーはオーバーレイの後ろに隠れていた。

**学び**: `Wait+Screen` は画面に実際に表示されている文字列のみマッチする。オーバーレイで隠れた部分は検出できない。オーバーレイ内のテキスト（例: `Export Results`）を Wait パターンに使う。

### asql は INSERT モードで起動する — VHS tape で `Type "i"` は不要

**文脈**: vim 風に `i` で INSERT モードに入る想定で tape に `Type "i"` を入れたところ、文字 `i` がエディタに入力されてしまった。

**学び**: asql は `mode: insertMode` で起動する（`model.go:230`）。起動直後は既に INSERT モードなので、`Ctrl+l` でエディタをクリアしてからクエリを入力する。

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

### sentinel 行がある場合の境界チェックは「データ行数」で判定する

**文脈**: `cellDiffAt` で `rowIdx >= selfCount || rowIdx >= len(selfRows)` としていたが、`selfCount`（実データ行数）≤ `len(selfRows)`（sentinel 含む表示行数）が常に成り立つため、`len(selfRows)` のチェックは冗長だった。Gemini bot レビューで検出。

**学び**: sentinel 行（`(no rows)` 等）を含む `displayRows` と実データ行数 `len(result.Rows)` が異なる場合、境界チェックは「実データ行数」側で行えば十分。表示行数との OR 条件は冗長で、意図を曖昧にする。

**パターン**:
```go
// NG: 冗長な二重チェック
if rowIdx >= selfCount || rowIdx >= len(selfRows) { ... }

// OK: データ行数だけで判定（sentinel は selfCount 未満にならない）
if rowIdx >= selfCount { ... }
```

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

### Markdown テーブルに改行文字を含むセルがあるとレイアウトが崩れる

**文脈**: `FormatMarkdown` でセル値の `|` はエスケープしていたが、`\n` / `\r\n` を処理していなかった。改行を含むセル（TEXT カラム等）があるとテーブル行が分断されて壊れた。

**学び**: Markdown テーブルは1行1レコードが前提。セル値に含まれる改行（`\r\n`, `\n`, `\r`）はスペースに置換してからパイプエスケープを行う。

**パターン**:
```go
escape := func(s string) string {
    s = strings.ReplaceAll(s, "\r\n", " ")
    s = strings.ReplaceAll(s, "\n", " ")
    s = strings.ReplaceAll(s, "\r", " ")
    return strings.ReplaceAll(s, "|", "\\|")
}
```

### エクスポートファイルのパーミッションは 0600、タイムスタンプはミリ秒精度

**文脈**: `SaveCSVFile` が `0644` でファイルを書き込み、タイムスタンプが秒精度（`20060102_150405`）だったため、(1) 他ユーザーに読まれるリスク、(2) 1秒以内の連続エクスポートでファイル上書きのリスクがあった。

**学び**: クエリ結果にはセンシティブなデータが含まれうるため、エクスポートファイルのパーミッションは `0600`（owner only）にする。ファイル名のタイムスタンプはミリ秒まで含めて衝突を防ぐ。

**パターン**:
```go
filename := fmt.Sprintf("result_%s.csv", time.Now().Format("20060102_150405.000"))
os.WriteFile(filename, data, 0600)
```

---

## リファクタリングパターン

### Bubble Tea の model 肥大化にはモード別状態構造体を抽出する

**文脈**: `model` 構造体に `detailFieldCursor`, `detailScroll`, `exportCursor`, `aiInput`, `aiLoading`, `aiError`, `completionActive`, `completionItems`, `completionCursor`, `completionPrefix`, `completionColCache`, `historySearchInput`, `historySearchResults`, `historySearchCursor`, `sidebarTables`, `sidebarCursor` 等が直接並んでおり、30 以上のフィールドになっていた。

**学び**: モードごとに関連するフィールドを構造体にグループ化すると、(1) model 定義の見通しが良くなる、(2) 名前の衝突を防げる（`cursor` が各モードにある）、(3) 初期化・リセットが構造体のゼロ値で済む。

**パターン**: `states.go` にモード別の状態構造体を定義し、model にはネストで持たせる:
```go
// states.go
type detailState struct { fieldCursor, scroll int }
type exportState struct { cursor int }
type completionState struct { active bool; items []string; cursor int; ... }

// model.go
type model struct {
    detail     detailState
    exportSt   exportState
    completion completionState
    // ...
}
```

### 重複するオーバーレイ描画は共通ヘルパーに抽出する

**文脈**: AI / Detail / Export / HistorySearch の各オーバーレイで `calcModalWidth` と `lipgloss.Place(...)` の呼び出しが4箇所に重複していた。幅計算のクランプ値（最小20）やセンタリングの `WithWhitespaceBackground` 設定がバラバラになるリスクがあった。

**学び**: 複数のオーバーレイで共通する「幅計算」と「背景上にセンタリング配置」は小さなヘルパー関数に抽出する。コード量は少ないが、一貫性の保証とバグの一箇所修正が目的。

**パターン**: `overlay.go` に2つの関数を置く:
```go
func calcModalWidth(screenWidth, maxWidth int) int {
    w := min(screenWidth-4, maxWidth)
    if w < 20 { w = 20 }
    return w
}

func overlayModal(screenWidth int, background, modal string) string {
    bgH := lipgloss.Height(background)
    return lipgloss.Place(screenWidth, bgH, lipgloss.Center, lipgloss.Center, modal,
        lipgloss.WithWhitespaceBackground(appBackground))
}
```

### カーソル移動の境界チェックは汎用ヘルパーで統一する

**文脈**: Export / Detail / Sidebar / Snippet / Profile の各モードで `if cursor < len(items)-1 { cursor++ }` / `if cursor > 0 { cursor-- }` が10箇所以上に重複していた。一部では `>= 0` と `> 0` の不一致や、空リストでの off-by-one リスクがあった。

**学び**: カーソル移動は「位置 + リスト長 + 方向」だけで決まる純粋な操作なので、1つの関数に統合できる。ポインタ渡しにすれば呼び出し側は1行で済む。

**パターン**:
```go
func moveCursor(cursor *int, length int, direction int) {
    n := *cursor + direction
    if n >= 0 && n < length {
        *cursor = n
    }
}
// 使用: moveCursor(&m.exportSt.cursor, len(exportOptions), 1)
```

---

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

## 非同期処理と接続管理 (2026-03-22)

### 非同期フェッチ結果には接続世代トークンを含める
- DB 接続切替中に非同期カラムフェッチが in-flight の場合、旧接続の応答が新接続のキャッシュを汚染する
- **ルール**: 非同期メッセージには `connGen uint64` を含め、ハンドラ側で `msg.connGen != m.connGen` なら破棄する。接続切替時に `connGen++` する

### 非同期応答での自動挿入は発行時のコンテキストを検証してから行う
- 非同期カラムフェッチの応答で `triggerCompletion()` を再実行すると、ユーザーが既にカーソルを移動していても single candidate の自動挿入が走り、意図しないテキスト変更が起きる
- **ルール**: 非同期フェッチ発行時に `pendingPrefix` を保存し、応答到着時にカーソル位置の prefix が一致する場合のみ re-trigger する。不一致なら `pendingPrefix` をクリアして無視

### 重複ユーティリティ関数は共通パッケージに切り出す
- `atomicWrite` が `profile.go` と `snippet.go` に全く同一のコードで重複していた。一方の修正がもう一方に反映されないリスク
- **ルール**: 2つ以上のパッケージで同一ロジックが必要な場合は `internal/` 配下に共通パッケージ（例: `fsutil`）を作り、テスト付きで切り出す

### SQLite の MaxOpenConns は 1 にする
- SQLite はシングルライターアーキテクチャ。`MaxOpenConns(5)` は他 DB アダプタからのコピペで設定されていたが、SQLite では不適切
- **ルール**: DB アダプタの接続プール設定はデータベースの特性に合わせる。SQLite は `MaxOpenConns(1)`, `MaxIdleConns(1)`

---

## Issue 対応とリファクタリング (2026-03-24)

### Issue 着手前に実装状況をコードで確認する
- Issue #16（DSN セキュリティ）を開いたところ、提案の3項目（環境変数・プロファイル・対話的入力）がすでに実装済みだった。コードを読まずに実装に入ると無駄な重複が生じる
- **ルール**: Issue に着手する前に、提案された機能がすでに存在しないか `grep` / `Explore` で確認する。既に実装済みであればコメント付きでクローズする

### model.go の分割は「モード別」より「責務別」が残りの軸になる
- Issue #13 が提案した「モード別分割」（normal.go, insert.go 等）はすでに完了済みだった。残った model.go（907行）は共通インフラのみで、さらに「責務別」に分割できた
- **ルール**: 大きなファイルを分割する際、モード/機能ごとの分割が完了したら次は「責務」軸で見直す。`sanitize.go`（純粋関数）・`query.go`（実行ヘルパー）・`result.go`（ビューポート管理）のような軸が有効
- **パターン**: 分割後のファイルが「単一の関心ごとを持つ」かチェックする。model 依存のない純粋関数は独立性が高く低リスクで移動できる

### 関数を別ファイルへ移動した後は移動元の import を精査する
- `dbutil` を使う関数を `result.go` に移したあと、`model.go` の import に `dbutil` が残りビルドエラーになった
- **ルール**: 関数を別ファイルへ移動したら、移動元ファイルの import を全行レビューし、使われなくなったものを削除する。`go build` が即座に検出するので、各ステップ後に必ず実行する

---

## テストカバレッジ拡充 (2026-03-25)

### UI テストでは状態遷移の複数ステップを t.Run サブテストに分割する
- `TestInsert_CtrlPNavigatesHistoryBack` で `*m = result` パターンで複数ステップを直列にテストしていたが、Gemini レビューで `t.Run` サブテスト化を指摘された
- **ルール**: 1つのテスト関数内で複数の状態遷移をテストする場合、各ステップを `t.Run` で分割する。失敗時にどのステップで問題が起きたか即座に特定できる
- **パターン**: 境界チェックテスト（CursorBoundary）も「上端」「下端」を別サブテストにする

### AI コードレビューの重複レビュー判定は慎重に
- PR #35 に gemini-code-assist のレビューが既にある状態で Claude Code レビューを実行した。適格性チェックで「既にAIレビューあり→スキップ」と判定されたが、異なるAIの視点は補完的であり、ユーザーが明示的にレビューを依頼した場合はスキップすべきでない
- **ルール**: `/code-review` をユーザーが明示的に呼んだ場合は、既存のbot レビュー有無に関係なく実行する

---

## Column Statistics 機能 (2026-03-25)

### 複数の軽量機能は1つのオーバーレイにまとめて実装する
- Phase 4 の 4-1 (NULL率), 4-2 (distinct数), 4-3 (min/max) を個別に実装するより、1つの Stats overlay にまとめた方がユーザー体験が良い
- **ルール**: 同じデータソース（クエリ結果）に対する複数の統計指標は、個別UIではなく1画面にまとめる。1キー (`d`) で全情報にアクセスできる

### インメモリ計算 vs DBクエリの判断基準
- Stats 機能で DB に `SELECT COUNT(DISTINCT col), ...` を投げる案と、`lastResult.Rows` からインメモリ計算する案を比較した
- **ルール**: TUI に既にデータがロード済みなら、インメモリ計算を優先する。理由: (1) DBラウンドトリップ不要で即座に表示、(2) 任意のクエリ結果に対して動作（テーブル名不要）、(3) 接続切断後も動作する

---

## Sparkline 機能 (2026-03-28)

### overlay で追加行を描画する場合、スクロール計算を統一する
- Stats overlay にスパークライン行を追加した際、`updateStats` と `renderWithStatsOverlay` で `maxVisible` の計算が異なり、カーソルが最下部で非表示になるバグが発生した
- **ルール**: overlay 内で条件付き追加行（詳細パネル等）を描画する場合、スクロール計算と描画で同じ `maxVisible` を使うこと。共通メソッド（例: `statsMaxVisible()`）に抽出して一箇所で管理する

### 日付型カラムの検出は ColumnTypes + 値サンプリングの2層で行う
- SQLite は TEXT 型に日付を格納することが多く、`ColumnTypes` だけでは検出できない。一方で値サンプリングだけだとコード列（`"2024-01-01"` のような値を持つ非日付カラム）で誤検出する
- **ルール**: まず `ColumnTypes` で `date`/`time`/`timestamp` を検出し、フォールバックとして先頭の非NULL値を `time.Parse` で試す。ヒューリスティックのトレードオフはコメントで明記する

### パフォーマンスガードは計算関数の先頭に置く
- スパークライン計算で 10,000行を超えるとスキップする仕様を実装。ガードを関数先頭に配置することで、不要なメモリ確保やパース処理を完全に回避できる
- **ルール**: 大量データの同期処理にはハードリミットを設け、関数の最初の行で `len(rows) > limit` をチェックして即 return する。スキップ理由はユーザーに表示する（silent fail を避ける）

---

## コード品質・パフォーマンス改善 (2026-04-04)

### 循環依存を避けるために「接着剤パッケージ」を使う
- `internal/db` に `OpenByDSN` を置くと `db` → `db/sqlite` → `db` の循環依存になる。サブパッケージのインターフェースを定義するパッケージからサブパッケージの具象を import できない
- **ルール**: インターフェース定義パッケージと具象実装パッケージを橋渡しする「接着剤パッケージ」（例: `internal/db/opener`）を作り、そこで import を集約する。呼び出し元（`main.go` 等）はこの接着剤パッケージだけを知ればよい

### 方言差分のある重複ロジックは Dialect オプションで共通化する
- SQLite と PostgreSQL の `containsReturning` が 80% 同一だが、SQLite は `[...]` と `` ` `` 対応、PostgreSQL は `$$` 対応という差分があった。各アダプタに ~90行の重複コードが存在
- **ルール**: SQL スキャナなど方言差分のある重複ロジックは、差分部分を `Dialect` 構造体のフラグで制御する共通関数にまとめる。`dbutil` に既にヘルパー（`skipSingleQuoted` 等）がある場合はそれを再利用する

### ScanRows に行数上限を設けて OOM を防ぐ
- `ScanRows` が全件を `[][]string` にメモリ読み込みしていた。巨大な SELECT 結果で OOM やフリーズのリスク
- **ルール**: DB 結果セットのスキャンにはデフォルト上限を設ける（10,000行）。`QueryResult.Truncated` フィールドで切り捨てを通知し、ステータスバーで表示する。上限なしが必要な場合は `ScanRowsLimit(rows, 0)` で明示的に指定

### Bubble Tea の重い同期計算は tea.Cmd で非同期化する
- Stats overlay の `computeColumnStats` が同期実行で、大結果セット時に UI がフリーズしていた
- **ルール**: `Update()` 内で結果セット全体を走査する計算は `tea.Cmd`（`func() tea.Msg`）で非同期化する。loading フラグと専用メッセージ型（例: `statsComputedMsg`）で結果を受け取る。既存テストは `cmd()` を呼んでメッセージを手動送信するパターンに更新する

### ライブラリ層から stderr に直接出力しない
- `config.Load()` がパーミッション警告を `fmt.Fprintf(os.Stderr, ...)` で直接出力していた。テスト困難で、TUI 起動後に画面が崩れるリスク
- **ルール**: ライブラリ関数は戻り値（フィールドまたは error）で情報を返す。stderr/stdout への出力は `main.go` など最上位の呼び出し元の責任

### API エラーレスポンスは構造化抽出してから返す
- AI クライアントが HTTP エラー時にレスポンスボディ全体をエラーメッセージに含めていた。秘密情報（APIキーエコーバック等）がユーザーに露出するリスク
- **ルール**: 外部 API のエラーレスポンスは (1) 既知の構造化形式（OpenAI の `{"error":{"message":"..."}}` 等）を試行抽出、(2) メッセージ長を制限（200文字）、(3) 取得不可時はステータスコードのみ返す

### Codex worktree 環境ではプラグイン state ディレクトリが read-only になる
- `isolation: "worktree"` で Codex サブエージェントを起動したが、プラグインの state ディレクトリが read-only マウントされており、ジョブログを書き込めず全タスクが失敗した
- **ルール**: Codex を Claude Code から呼ぶ場合は worktree 分離を使わない。メインリポジトリのコンテキストで実行するか、worktree なしのサブエージェントで投げる

---
