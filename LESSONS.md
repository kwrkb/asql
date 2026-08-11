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

**学び**: asql は `mode: insertMode` で起動する（`internal/ui/model.go` の初期 model 構築箇所）。起動直後は既に INSERT モードなので、`Ctrl+l` でエディタをクリアしてからクエリを入力する。

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

**パターン**: `overlay.go` に2つの関数を置く（`calcModalWidth` の下限は実画面幅でクランプし、極端に狭い端末でもはみ出さないようにする）:
```go
func calcModalWidth(screenWidth, maxWidth int) int {
    w := min(screenWidth-4, maxWidth)
    minWidth := min(20, max(screenWidth, 1))
    if w < minWidth {
        w = minWidth
    }
    return w
}

func overlayModal(screenWidth int, background string, modal string) string {
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

## リリース運用 (2026-04-25)

### goreleaser v2 では archives.format ではなく formats を使う
- v0.10.0 リリース時に `format: tar.gz` / `format_overrides[].format: zip` で `DEPRECATED: archives.format should not be used anymore` の警告が出た。goreleaser v2 では `format` (単数) が `formats` (複数形・配列) に置き換わっている
- **ルール**: `.goreleaser.yml` の archives は `formats: [tar.gz]` / `format_overrides[].formats: [zip]` で記述する。設定変更後は `goreleaser check` で deprecation を事前検証してからタグを切る

### リリース前に PLAN.md / HISTORY.md を更新してから push & タグを切る
- タグはコミットを指すので、PLAN/HISTORY が未コミットのままタグを打つとリリース成果物にドキュメントが含まれない。事前に `git status` クリーンを確認しないと後追い不可
- **ルール**: リリース手順は (1) PLAN/HISTORY 更新→コミット→push、(2) `git status` クリーン確認、(3) `go vet ./... && go test ./...`、(4) `git tag -a vX.Y.Z -m "vX.Y.Z"` → `git push origin vX.Y.Z`、(5) `GITHUB_TOKEN=$(gh auth token) goreleaser release --clean` の順で固定する

### goreleaser はシステムに無ければ go install で入れる
- WSL/Linux 環境に goreleaser のバイナリは未配備で、apt にも公式パッケージはない
- **ルール**: `go install github.com/goreleaser/goreleaser/v2@latest` で `$(go env GOPATH)/bin/goreleaser` に入る。`~/go/bin` が PATH に無ければ絶対パス `~/go/bin/goreleaser` で実行する

---

## TUI レイアウト・モード遷移 (2026-04-30)

### Bubble Tea の View() は純粋関数として保つ
- `renderCompareView` が `View()` 経路から `syncPinnedTable` を呼んでテーブル状態を変異させていた。レンダリングごとに pinned テーブルが書き換わり、不定期な再描画バグの温床になる
- **ルール**: View() および View() から呼ばれる関数は読み取り専用にする。テーブル/ビューポート同期は `Update`・`resize`・結果適用時 (`applyResult` 後) などのイベントパスに移し、共通ヘルパー（例: `syncCompareTables()`）に集約する

### TUI 幅計算では `available <= 0` と固定オフセットの負値を必ずガードする
- `visibleColumnRange` が `available := contentWidth() - 8` を負/ゼロでもループに突入し、最初のカラムを強制描画してレイアウト崩れを起こしていた。同様に `maxVisible := height - 2` もクランプなしで負値になり得た
- **ルール**: `width - N` / `height - N` などのオフセット計算は (1) 結果が `<= 0` のとき早期 return か空範囲を返す、(2) 後段で使う場合は `max(value, 0)` または `max(value, 1)` でクランプする。1行の `max()` で済むのでケチらない

### モーダル overlay の入力幅は `resize()` で `calcModalWidth` から同期する
- AI/Snippet/Profile overlay の `textinput.Width` が `50`/`30` の固定値で、`modalWidth` の縮小に追従せず狭画面で入力欄がモーダルからはみ出していた。一度は `renderWith*Overlay` 内で `m.xxxSt.input.Width = ...` を設定して直したが、これは「View() は純粋関数として保つ」ルールに反する状態変異だった
- **ルール**: モーダル入力幅の同期は `resize()` の末尾に集約し、`m.xxxSt.input.Width = max(calcModalWidth(m.width, N)-K, min)` を一括で更新する。`renderWith*Overlay` は `modalWidth` を計算しても `input.Width` には書き込まない（読み取りのみ）。`calcModalWidth` 自身も `min(20, max(screenWidth, 1))` で実画面幅を下限に取る

### モード遷移時の Blur は専用ヘルパーで集約する
- Ctrl+C グローバルキャンセル時、`m.textarea.Blur()` だけ呼んでいたため AI/Snippet/Profile/HistorySearch モードの input がフォーカス残留していた
- **ルール**: `blurActiveInput()` のように現在の mode に対応する input を Blur する単一メソッドを用意し、グローバルキャンセル・mode 遷移直前に呼ぶ。新しい入力フィールドを持つモードを追加したら必ずこのヘルパーに分岐を追加する

### Codex の sandbox モードは `--resume` では切り替わらない
- 前回 read-only sandbox で起動した Codex タスクを `--resume` で再開しても read-only のままで、ファイル書き込みが必要なフォローアップタスクが失敗した
- **ルール**: 前回タスクが調査・読み取りで起動していた場合、書き込みを伴う続行タスクは `--fresh` で再投入する。Codex は前回スレッドの分析を保持しているので、同じ指示を新スレッドで投げ直して問題ない

---

## PR レビュー対応 (2026-04-30)

### LESSONS.md にルールを追記する PR では、同じ PR 内に違反箇所が残っていないかを push 前に必ず grep する
- PR #44 で「View() は純粋関数として保つ」ルールを LESSONS.md に追記したが、同 PR の `renderWithAIOverlay` / `renderWithSnippetOverlay` / `renderWithProfileOverlay` / `renderWithHistorySearchOverlay` 内で `m.xxxSt.input.Width = ...` を書き続けていた（= ルール違反 4 箇所）。gemini-code-assist bot の review-comment で 4 件まとめて指摘され、追加コミット (`ee39ea6`) で `resize()` 集約に直す羽目になった
- **ルール**: ルールを LESSONS.md に追記したら、push する前に「ルール本文で禁じている書き方」（今回の例: `View()` 経路で `.input.Width =` / `.SetXXX(`）を `rg` で全検索し、違反候補を同 PR 内で全部潰す。レビューに通す前に自分で気づくこと

### bot レビューでも「自家製ルール / 既存コードへの具体的参照」を含む指摘は人間レビュー同等に扱う
- gemini-code-assist の指摘は `LESSONS.md (661行目) のルールに違反` と参照先を行番号で示しており、内容は完全に正確だった。「bot だから」とフィルタすると、自分で書いたばかりのルールへの違反を放置することになる
- **ルール**: bot 指摘の優先度判断は「指摘が *この repo の* 規約・ファイル・行番号を具体的に参照しているか」で切り分ける。具体参照ありなら人間レビュー同等で対応、一般論的な命名・抽象化提案だけならスコープ外として却下してよい

### `resolve-pr-comments` 後はマージ可否を `mergeStateStatus` で確認してから `--auto` でマージ予約する
- PR #44 のレビュー対応コミットを push 直後、`mergeable: MERGEABLE / mergeStateStatus: UNSTABLE`（CI 進行中）の状態だった。手動で merge を待つより `gh pr merge --squash --auto --delete-branch` で予約するほうが、CI 完了即マージ＋ローカル/リモート両ブランチ削除まで一発で済む
- **ルール**: レビュー対応 push 直後にマージしたい場合は (1) `gh pr view <N> --json mergeable,mergeStateStatus` で `MERGEABLE` を確認、(2) CI 進行中なら `gh pr merge <N> --squash --auto --delete-branch` で auto-merge 予約、(3) 完了後 `git fetch --prune` で remote-tracking 参照を整理する

---

## Phase 3 着手: Bring & Join (2026-07-01)

### スキャン時点で型情報を失う設計では、持ち寄り先DBは全TEXT・文字列一致JOINを選ぶ
- `dbutil.ScanRowsLimit`/`StringifyValue` が scan の時点で全セルを `[][]string` へ変換しており（`nil→"NULL"`、空文字列→`""` の表示センチネル等）、`QueryResult` 到達後にはどこにもタイプ付きの値が残っていない。元DBへの再クエリやColumnTypesからのSQLite型マッピングは、クロスDB型名の差異吸収とNULL曖昧性解消のどちらも完全には果たせない割に実装コストが大きい
- **ルール**: 表示用に文字列化済みのデータを別DBへ持ち寄る機能を作るときは、まず「型を厳密に復元する」を最初の選択肢にしない。全カラムTEXT・文字列比較という軽量実装を基本案として提示し、数値比較やNULL判定の曖昧さを既知の制約として明示した上でユーザー承認を取ってから実装する

### 新機能のために新しい Bubble Tea モードを安易に追加しない
- Bring & Join の実装で「ローカル一時DBへの保存」と「JOIN実行」のために専用モード・専用テキストエリアを作る案もあったが、ローカルSQLite DBを `connManager` の「もう一つの接続」として登録し、既存のクエリ実行パイプライン（INSERTモード→`executeQueryCmd`→`applyResult`）をそのまま使う設計にしたことで、Stats/Export/Sort/Compare が無改修でJOIN結果にも効くようになった
- **ルール**: 新機能が「既存のクエリ実行・表示パイプラインに乗せられるデータ」を生成するだけなら、新しい `mode`/オーバーレイを足す前に「既存の接続/実行経路に統合できないか」を先に検討する。新モード追加のコスト（`states.go`/`model.go`/`Update`/`View`/`blurActiveInput`/`resize` 全箇所への波及）は、既存パイプラインへの統合コストと比較してから選ぶ

### `connManager.Switch` に未登録のダミーDSNを渡すと、内部生成DBが二重に開かれてデータが消える
- `Switch(name, dsn)` は既知の `dsn` が `conns` に無い場合 `opener.Open(dsn)` を呼ぶ。`opener.Open` は `DetectType` で mysql/postgres と判定できない文字列をすべて sqlite 扱いするため、内部生成した一時SQLite DB（`:memory:`等、正規のDSNを持たない）を識別する目的でダミーDSN文字列を渡して `Switch` を呼ぶと、既存の接続と一致せず新規に空の `:memory:` DBが開かれてしまい、それまで投入したデータが消失する
- **ルール**: アプリ内部で生成した `db.DBAdapter`（`opener.Open` を経由しない接続）を `connManager` に登録するときは、`Switch` 経由ではなく専用の `Register(name, dsn, adapter)` のような「既存 `conns` に直接追加するだけのメソッド」を使う。以降の再アクティブ化は固定の合成DSN定数で `Switch` を呼べば、既存エントリにマッチして再オープンされない

### sqlite3 CLI が無い環境でのフィクスチャDB作成とTUI手動smoke testの手順
- WSL環境に `sqlite3` CLI が入っておらず、TUI機能の手動検証用フィクスチャDBを作る手段に詰まった
- **ルール**: (1) フィクスチャDB作成は `database/sql` + `_ "modernc.org/sqlite"` を使った使い捨ての `go run` スクリプトで `CREATE TABLE`/`INSERT` を直接実行する（プロジェクトの go.mod に既にドライバ依存があるので追加インストール不要）。(2) TUIの対話操作は `tmux new-session -d` でバックグラウンド起動し、`tmux send-keys` でキー入力（`C-l`/`C-j`等のctrlキーや文字列）を送り、`tmux capture-pane -p` で画面内容を読み取って検証する。プロファイル等で複数DB接続が必要な場合は `XDG_CONFIG_HOME` を一時ディレクトリに向けて `profiles.yaml` を事前投入すると、本番の `~/.config/asql/` を汚さずに済む

### /code-review xhigh で Bring & Join PR から見つかった設計バグ（手動smoke testでは踏めなかったもの）
- PR作成後の `/code-review xhigh --fix` で、tmux smoke test（正常系1往復のみ）では踏めない8件の欠陥が見つかった。いずれも「正常系は動くが異常系・境界値・時間経過で壊れる」パターン
- **CREATE TABLE を autocommit のまま INSERT と別トランザクションにしない**: `Materialize` が `createTable` を単独実行し `insertRows` だけを別トランザクションにしていたため、INSERT失敗時にテーブルだけが残り、呼び出し側のリトライ用カウンタを巻き戻しても同名テーブルが既に存在し永久に失敗する。**ルール**: 「作成→投入」を1つの操作として提供するAPIは、内部を1つのトランザクションに包み、失敗時は作成側も含めて全ロールバックする
- **バッチサイズは「行数」ではなく「行数×列数（バインドパラメータ数）」で制限する**: 固定行数バッチ（500行/文）は列数を考慮しておらず、列数の多い結果（70列×600行など）でSQLiteのバインドパラメータ上限（約32766）を超えて失敗した。**ルール**: 可変長データをバッチINSERTするAPIのバッチサイズは `min(固定上限, パラメータ上限 / 列数)` のように列数を考慮して動的に決める
- **`:memory:` DBを保持する接続に `SetConnMaxLifetime` を設定しない**: 汎用の `sqlite.NewAdapter` が5分の接続寿命を設定しており、ファイルDBなら再オープンで復元できるが、プライベートな `:memory:` DBでは寿命切れによる接続クローズ＝データ消失になる。**ルール**: `:memory:` を単一接続で保持するようなラッパーでは、汎用アダプタのコネクションプール設定をそのまま信頼せず、寿命ゼロ（無期限）に上書きする
- **重複列名の disambiguate は大文字小文字非依存で行い、生成した代替名が実在する別列名と衝突しないようにする**: SQLiteは列名を大文字小文字区別しないため `"ID"`/`"id"` の衝突を見逃すと `CREATE TABLE` が失敗する。また `["id","id","id_2"]` のような入力でナイーブに連番を振ると、本物の `id_2` 列を上書きし別の列のデータを指すようになる（サイレントに誤ったデータを返す、失敗より悪いバグ）。**ルール**: 名前の重複解消アルゴリズムは (1) 大文字小文字を正規化して比較する (2) 元の全列名を「予約済み」として先に確保し、生成する代替名がそれと衝突しないように追加でスキップする
- **DBクエリの実データ書き込み処理は Bubble Tea の `Update` ループ内で同期実行しない**: 既存のクエリ実行（query.go）は `tea.Cmd` 経由の非同期だったが、新規追加した bring 機能の `Materialize` 呼び出しは見落として `Update` 内で同期実行していた。単一コネクション上で別クエリがbusyな間に押すとUI全体が最大 `queryTimeout` 秒フリーズする。**ルール**: 新しいDB書き込み処理を追加する際は、既存のDB呼び出しが同期か非同期かを確認し、既存パターン（`tea.Cmd` を返して結果を独自の `xxxDoneMsg` で `Update` に返す）に合わせる
- **内部生成した合成DSNは、ユーザーが「現在の接続を保存」できる導線から見えないようにガードする**: bring用のダミーDSN（`asql-bring`）が `rawDSN` として外部から見える状態のままだったため、プロファイル保存機能でそのまま永続化でき、後日 `opener.Open` がそれをファイルパスとして解釈しカレントディレクトリに空ファイルを作ってしまう経路があった。**ルール**: アプリ内部専用の合成DSN/識別子を導入したら、ユーザーがそれを外部化できる導線（保存・エクスポート・共有など）を洗い出し、該当箇所に明示的なガードを入れる
- **手動smoke testは正常系1往復では検証済みと言えない**: tmuxでの手動確認は「2DBから持ち寄ってJOINが通る」ことしか検証しておらず、上記の異常系・境界値・時間経過バグは一切踏めていなかった。**ルール**: 手動smoke testは「動くことの確認」であって「バグがないことの証明」ではない。実装完了後は `/code-review`（xhigh以上）などの独立レビューを通してから初めて「検証済み」と report する

### 同期処理を非同期(`tea.Cmd`)化する修正自体が新しい競合バグを生むことがある
- xhighレビュー対応で `bring.Materialize` を同期実行から `tea.Cmd` 非同期実行に変更したところ、bot(chatgpt-codex-connector)の追加レビューで「複数の bring 操作が同時に in-flight のとき、失敗時の `tableSeq--` が後発の成功済み採番と衝突しうる」という新規バグが指摘された。同期実行時は暗黙的に直列化されていたため存在しなかった競合が、非同期化によって顕在化した
- **ルール**: 同期→非同期(`tea.Cmd`)化のリファクタは「その処理が使うグローバルな可変状態（カウンタ等）が複数同時実行を前提に安全か」を必ず再検証する。特に「成功/失敗に応じてカウンタを増減させる」ロジックは、複数の呼び出しが完了順不同で戻ってくる前提で設計し直す。今回は「失敗時はロールバックせず番号を消費したままにする」（モノトニックカウンタ + 名前の使い捨てを許容）という設計にすることで、ロールバックによる巻き戻し自体をなくし競合の可能性を根本から排除した
- **bot指摘は「その指摘が今のコードに対して有効か」をまず現在のソースで確認してから採否を決める**: gemini-code-assistの「`sqlite.NewAdapter` エラー時に `conn` がリークする」という指摘は、`newAdapter` の唯一のエラーパス(`PingContext`失敗)で既に `conn.Close()` されており誤検知だった。bot提案のコードをそのまま貼る前に、指摘対象の関数を実際に読んでトレースする一手間で誤修正を避けられた
- **PRに複数のレビューbot（gemini-code-assist, chatgpt-codex-connector等）が付いている場合、同じ指摘が重複して出ることがあるので実装状態と突き合わせて重複排除する**: 今回は「バッチサイズを列数で動的化せよ」「非同期化せよ」等がgemini/codex双方から出ていたが、直前のコミットで既に対応済みだった。コメント本文に含まれるコミットハッシュ（Reviewed commit）を見て、どのコミット時点の指摘かを確認してから要否判断する

---

## 定期メンテナンス

### `go vet`/`go test` が全パスでも `govulncheck` は別途実行しないと到達脆弱性を見逃す
- 定期メンテナンス点検で `go vet ./...` / `go test ./...` は問題なしだったが、`govulncheck ./...` で `github.com/jackc/pgx/v5 v5.9.1` の到達可能な脆弱性（GO-2026-5004、v5.9.2で修正済み）が検出された。CLAUDE.mdの「Go Security Scan」ルール通り、リリース前だけでなく定期メンテナンス時にも `govulncheck` を独立して回す必要がある
- **注意**: govulncheckの呼び出しトレースは、インターフェース（動的ディスパッチ）に対する静的解析の過剰近似（over-approximation）により、無関係なファイル（例: `sqlite/adapter.go`）経由の呼び出しとして表示されることがある。トレースの妥当性を検証しようとせず、対象モジュールを修正版に上げて再実行し「0件」になることだけを確認すればよい
- **ルール**: 依存更新時は `go get <pkg>@<fixed-version>` → `go mod tidy` の後、`go.mod` の `go`/`toolchain` ディレクティブが意図せず引き上げられていないか diff で確認する（本プロジェクトはPR #42で `go 1.26.2` に意図的にpin済み）

---

## Bring & Join の意味保存 (2026-08-11)

### 「型を厳密に復元しない」という過去の判断は、復元先の要件が変わった時点で覆す
- LESSONS.md の「スキャン時点で型情報を失う設計では、持ち寄り先DBは全TEXT・文字列一致JOINを選ぶ」（2026-07-01）は、当時「元DBへの再クエリ」と「ColumnTypes からのSQLite型マッピング」の2案を検討して却下した記録だった。今回の実装はそのどちらでもない第3の経路 — **スキャン時点でセルごとに1バイトの `Kind` を記録する** — で、当時の却下理由（クロスDB型名の差異吸収ができない／NULL曖昧性が解けない）を両方回避した
- **却下した案**: (a) `Rows` を `[][]Value` に変更する。sort/stats/export/detail/compare と全テストに波及し、`[][]string` を前提にした表示経路を全て書き換える必要がある (b) 列単位の型情報だけ持つ。NULL はどの列にも現れるのでセル単位の情報が要る (c) 表示センチネル（`"NULL"`/`""`）を描画層へ移す。sort.go / stats.go / export / detail の4経路に波及する
- **決め手**: `Kinds [][]db.Kind` を `Rows` と並走させると、`Rows [][]string` が一切変わらないため描画・ソート・エクスポート・比較・詳細表示が無改修で済み、変更が3ファイル（`adapter.go` / `dbutil.go` / `bring.go`）に収まった。`KindText` を zero value にしたので、`Kinds` を持たない既存の `QueryResult` リテラル（テスト内に68箇所）は全て無変更で従来挙動にフォールバックする
- **覆す条件**: `Kinds` を参照する消費者が bring と stats 以外に3つ以上増えたら、`Rows`/`Kinds` の並走インバリアント（並べ替え・フィルタ時に両方を同じ順序で変換する義務）がバグの温床になる。その時点で `[][]Value` への移行を再検討する

### SQLite の列 affinity は「宣言した型」より弱いが、宣言しないことが最も強い選択になる場合がある
- 混在列（int と text が混ざる列）を `TEXT` と宣言すると、`int64` を bind しても SQLite が text へ強制変換し、型を運んできた意味が消える。実測: `INSERT INTO ta(v TEXT) VALUES (?)` に `int64(42)` → `typeof(v) = text`
- 型を宣言しない列（＝BLOB affinity）は各値の storage class をそのまま保持する。実測: 同じ値の並びで `typeof` が `integer` / `real` / `text` に分かれ、`ORDER BY` も NULL < 数値 < テキストの順で正しく並ぶ
- **ルール**: 異種のデータをSQLiteへ持ち寄るとき、列の型宣言は「その列の全値が同じ storage class のときだけ」行う。混在するなら宣言しない。「とりあえずTEXT」は最も情報を失う選択肢

### affinity は「記録した型」ではなく「実際に bind する値」から決める
- `KindInt` として記録した `18446744073709551615`（uint64 max）は `strconv.ParseInt` で int64 に収まらない。この列を `INTEGER` と宣言すると、text として bind しても SQLite の INTEGER affinity が数値変換を試み、**REAL の `1.8446744073709552e+19` になって精度が落ちた**（テストで検出）
- **ルール**: 「型情報から列定義を決める」と「値を型付きで bind する」を別々のロジックで書かない。両方が同じ判定関数（今回は `effectiveKind`）を通るようにして、宣言と実際に入る値が食い違わないようにする。パースを2回走らせるコストは、宣言と値の不整合というサイレントなデータ破損より安い

### provenance は新しいUIではなく、持ち寄り先DB内の普通のテーブルとして持たせる
- 「t1 がどこから来たか」を見せる案として (a) サイドバーへの表示 (b) 専用オーバーレイ (c) ステータスバー を検討した。(a) は `sidebarWidth = 25` に `t1  users@prod 1.2k` が入らない。(b) は新モード追加のコスト（`states.go`/`model.go`/`Update`/`View`/`blurActiveInput`/`resize` への波及）に見合わない
- **決め手**: bring DB 内に `_asql_bring` テーブルを作れば、既存のクエリ経路でそのまま `SELECT`・ソート・エクスポート・JOIN でき、UIコードの追加がゼロになる。先頭アンダースコアでサイドバー先頭に並ぶので発見もされる。ステータスバーには「何個持ち寄ったか」という一行の状態だけ出す
- **ルール**: 「保存したデータについてのメタデータ」を見せたくなったら、専用UIを作る前に「そのデータと同じ場所に、同じ方法で読めるテーブルとして置けないか」を先に考える

### 接続レベルの readonly は SQLite でも手段によって強度が違う（readonly mode 設計時の実測）
- `file::memory:?_pragma=query_only(1)` は `INSERT` を拒否するが、ユーザーが `PRAGMA query_only=0` を実行すれば**解除できてしまう**
- `file:<path>?mode=ro` は `PRAGMA query_only=0` を実行しても解除されない。ただし `ATTACH DATABASE` は成功するので、別DBを繋いでそこへ書くことは可能
- **ルール**: 「DBドライバの readonly オプションを付けたから安全」と書く前に、(1) そのオプションをSQLから解除できないか (2) 別のリソースを開く経路が残っていないか を実際に叩いて確かめる。接続レベルのガードは常に belt-and-braces であって、SQL guard の代替にはならない

### 幅計算をするサードパーティ widget に、スタイル付き文字列を渡さない
- 型情報保持の作業中に TUI の smoke test をしたところ、カラムヘッダの型注釈が出ていないことに気づいた。変更前のコミットをビルドして同じ現象を確認し、既存バグと判明。原因は `bubbles/table` が `runewidth.Truncate(title, col.Width, "…")` を使っていることで、`runewidth.StringWidth` は ANSI エスケープのバイトを表示幅として数える（`"id \x1b[38;2;147;163;184mint\x1b[0m"` は可視6セルに対して runewidth では27）。列幅が27未満だとエスケープの途中で切断され、端末が残りを飲み込んで**中身ごと消える**
- 幅が広い列（`columnWidth` の上限32）では無傷で描画されるため、長期間気づかれなかった。同じ理由で比較モードの差分セルハイライトも狭い列で内容が消えている
- **却下した案**: 列幅を27以上に引き上げて回避する。160桁の端末で表示できる列数が12→5に落ち、「多カラムの観察」を殺す
- **決め手**: 上流に修正はない（bubbles v1.0.0 が最新、table.go はエスケープ対応 truncate をテストでしか使っていない）。修正には「table に ANSI を渡さない（＝色を諦める）」か「表の描画を自前で持つ」かの選択が要り、どちらも見た目の設計判断を含む。今回のスコープ（型情報保持・provenance・README同期）とは別件なので、`PLAN.md` に証拠付きで起票して見送った
- **ルール**: 幅計算・切り詰めを自前で行うライブラリにスタイル済み文字列を渡すときは、そのライブラリが `lipgloss.Width` 相当のエスケープ対応幅計算を使っているかを確認する。`runewidth.StringWidth` と `lipgloss.Width` の戻り値が一致するかを1行のテストで比べれば判別できる。**なお lipgloss は TTY でない環境では色を出さないため、この種の再現テストは `lipgloss.SetColorProfile(termenv.TrueColor)` を明示しないと素通りする**
- **覆す条件**: bubbles/table がエスケープ対応 truncate に移行したら、この制約は消える
