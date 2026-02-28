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
