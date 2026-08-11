# readonly mode — 設計案

**ステータス**: 設計のみ。未実装。
**日付**: 2026-08-11

本番DBを観察用途で安全に開くための readonly モードの設計。実装前にこの文書のレビューを受けること。

## 目的と非目的

**目的**: `asql --readonly @production` で本番DBに繋いだとき、うっかり `DELETE` を実行してしまう事故を防ぐ。

**非目的**: 完全なSQLサンドボックス。悪意ある利用者が回避できないことは保証しない。守るのは「意図しない書き込み」であって「意図的な書き込み」ではない。この線引きを守らないと、方言ごとの全構文を追いかける終わりのない作業になり、VISION の「軽さ」に反する。

## 二層構成

readonly は二層で構成する。どちらか一方では足りない。

| 層 | 役割 | 限界 |
|----|------|------|
| **SQL guard**（主）| asql が発行する前に文を分類して拒否する | 方言の全構文は網羅できない |
| **接続レベル**（従）| DBドライバ/サーバ側で書き込みを拒否させる | DBによって強度が違う。SQLite以外は未検証 |

### 層1: SQL guard（主）

**強制ポイント**は `db.DBAdapter` のラッパ1箇所。

```
ui.executeQueryCmd → adapter.Query(ctx, query)
```

`internal/db/readonly` パッケージで `DBAdapter` をラップし、`Query` の先頭で分類する。ここが唯一の実行経路なので:

- AI生成SQL（`Ctrl+K`）は textarea に挿入されるだけで、実行は必ずこの経路を通る → **追加対応不要で同じ制約がかかる**
- サイドバーの `SELECT * FROM ...` 挿入、スニペット、履歴も同様

`Tables` / `Columns` / `Schema` はアダプタ内部の固定クエリなので素通しでよい。

**分類は文字列 prefix 判定に依存しない。** 既存の `internal/db/dbutil` にあるスキャナを再利用する:

- `LeadingKeyword` — コメント（`--` / `#` / `/* */`）と先頭セミコロンを飛ばして最初のキーワードを取る
- `CteBodyKeyword` — `WITH ... ` の本体キーワードを、括弧深度・文字列リテラル・ドル引用符を追跡しながら取る（`WITH x AS (SELECT ...) DELETE FROM y` を `select` と誤認しない）
- `ContainsReturning` — 方言別の引用符スタイルを考慮した語境界スキャン

分類ルール（許可リスト方式。**未知のキーワードは拒否**）:

| 判定 | 扱い |
|------|------|
| `select` / `values` / `table` / `explain` / `show` / `describe` / `desc` | 許可 |
| `with` → `CteBodyKeyword` が上記のいずれか | 許可 |
| `with` → それ以外 | 拒否 |
| `pragma` | **後述の部分許可** |
| 上記以外（`insert`/`update`/`delete`/`drop`/`alter`/`create`/`truncate`/`replace`/`merge`/`grant`/`attach`/`vacuum`/…） | 拒否 |
| 空・キーワード抽出失敗 | 拒否 |

加えて **複文の拒否** が必要。現状の `LeadingKeyword` は先頭しか見ないので、`SELECT 1; DELETE FROM t` は先頭が `select` で通ってしまう。文字列リテラル・引用識別子・コメントを飛ばしながらセミコロンを探し、末尾セミコロン以外の区切りが1つでもあれば拒否する。`dbutil` のスキャナ（`skipSingleQuoted` 等）が既にあるので、それを使う小さな関数を1つ足せばよい。

> 補足: SQLite は複文を `Exec` でしか実行しないので現状でも `Query` 経由では通らないが、MySQL は `multiStatements` DSN パラメータ次第、PostgreSQL は simple protocol で通る。方言に依存させず guard 側で一律に拒否するのが正しい。

### PRAGMA の扱い

要件は「PRAGMA等、読み取り系は許可」だが、SQLite の `PRAGMA` は読み書き両用である。**引数なしの PRAGMA だけを許可する。**

- `PRAGMA table_info(users)` — 引数はあるが読み取り専用。許可したい
- `PRAGMA query_only=0` — 層2のガードを自分で解除できる。**必ず拒否**
- `PRAGMA journal_mode=DELETE` — 書き込み

したがって判定は「`=` を含まない PRAGMA のみ許可」。関数形式 `PRAGMA foo(bar)` の中に `=` は通常入らないので、この単純な規則で `table_info` / `foreign_key_list` / `index_list` 系は通り、設定変更形式は落ちる。厳密ではないが、非目的の線引き（サンドボックスを目指さない）と整合する。

### 層2: 接続レベル（従）

**実測結果**（modernc.org/sqlite v1.53.0、この環境で検証済み）:

| 手段 | 結果 |
|------|------|
| `file:<path>?mode=ro` | `INSERT` → `attempt to write a readonly database (8)`。**`PRAGMA query_only=0` を実行しても解除できない** |
| `file::memory:?_pragma=query_only(1)` | `INSERT` は拒否されるが、**`PRAGMA query_only=0` で解除できてしまう** |
| `mode=ro` + `ATTACH DATABASE '...' AS side` | **ATTACH は成功する**（別DBは書き込み可能になる） |

→ SQLite は `mode=ro` を使う。`_pragma=query_only` は単独では信頼できない。`ATTACH` が通る以上、層1で `attach` を拒否することは必須。

`mode=ro` は `file:` URI 形式を要求するため、現在の `sqlite.Open(path)` が受ける素のパスを URI へ変換する必要がある。パスに `?` や `#` を含むケースのエスケープに注意（`file:` + `url.PathEscape` 相当）。

**MySQL**: go-sql-driver/mysql v1.10.0 は DSN の未知パラメータを system variable として解釈し、**新規コネクションごとに `SET` を発行する**（README "System Variables"）。したがって `?transaction_read_only=1` はプール内の全コネクションに効く。アドバイザが懸念した「一度きりの `SET SESSION` はプールの1本にしか効かない」問題は、DSNパラメータ経由なら回避できる。**ただし実サーバでの動作は未検証**（この環境にMySQLがない）。

**PostgreSQL**: pgx は未知の接続パラメータをサーバの runtime parameter として渡すので `?default_transaction_read_only=on` が効くはず。こちらも**未検証**。

結論: 層2は SQLite でのみ検証済み。MySQL/PostgreSQL は実装時にライブサーバで確認し、確認が取れない場合は層1のみで出荷してよい（層2はあくまで belt-and-braces）。**「readonly接続だから安全」とREADMEに書かないこと。**

## 配線が必要な箇所（実装コストの実体）

guard 本体は小さい。コストはこちらにある。

1. **`internal/ui/connmgr.go` の `Switch(name, dsn)`** — `opener.Open(dsn)` を呼ぶが readonly を知らない。プロファイル単位の readonly を実現するには、readonly フラグを `Switch` に通すか、`connManager` に「このセッションは readonly」という状態を持たせてラップを `Switch` 内で行う必要がある。
   - セッション全体が readonly（`--readonly`）なら `connManager` にフラグを1つ持たせるのが最小。
   - プロファイル単位（`readonly: true`）なら、`connection` 構造体に持たせて `Switch` の引数を増やす。呼び出し元は `profile.go` の2箇所と `bring.go`。

2. **`Register()` 経由の bring DB は readonly にしない。** ローカルの持ち寄りDBは書き込めなければ機能しない。`Register` は `opener.Open` を通らないので、何もしなければ自動的に writable のまま — これは偶然ではなく意図として明記する。
   - 副作用として、readonly セッションでも `b` で結果を持ち寄って `J` でローカルJOINできる。観察ツールとして正しい挙動。

3. **CLI**: `parseSaveProfile` は手書きの引数パーサで、`resolveDSN` は `len(args) > 2` を弾く。`--readonly` は `resolveDSN` に渡る前に取り除く必要がある。`helpText` も更新する。

4. **`profile.Profile`** に `Readonly bool \`yaml:"readonly"\`` を足す。既存の `profiles.yaml` は当該キーなし → `false` になるので後方互換。

5. **状態の可視化**（VISION「状態が常に可視化されている」）— ステータスバーの接続ラベルに印を付ける。`statusConnectionLabel()` が `[name:SQLITE]` を返しているので `[name:SQLITE ro]` にする。色を増やさない。**これは省略不可**: 読み取り専用であることが見えないなら、事故を防ぐという目的の半分を失う。

6. **拒否時のメッセージ** — ステータスバーにエラー表示。`readonly: INSERT is not allowed (asql --readonly)` のように、何が拒否されたかとなぜかを1行で。

## テスト方針

- `internal/db/readonly` のテーブル駆動テスト: 許可/拒否の分類。特に
  - `WITH x AS (SELECT 1) DELETE FROM t` → 拒否
  - `SELECT 1; DELETE FROM t` → 拒否
  - `/* comment */ -- x\n SELECT 1` → 許可
  - `SELECT 'DELETE FROM t'` → 許可（文字列リテラル内）
  - `PRAGMA table_info(x)` → 許可 / `PRAGMA query_only=0` → 拒否
  - `ATTACH DATABASE ...` → 拒否
- ラッパが `Tables`/`Columns`/`Schema` を素通しすることの確認
- SQLite の `mode=ro` 実接続で `INSERT` が層2でも落ちることの確認

## 見積り

| 項目 | 規模 |
|------|------|
| `internal/db/readonly` パッケージ + テスト | 中（分類ルールの正確さがすべて） |
| アダプタラッパ | 小 |
| connmgr / profile / CLI 配線 | 中（ユーザー向け設定面に触るのでレビュー要） |
| ステータスバー表示 | 小 |

## 決定が必要な点

1. `--readonly` は**セッション全体**か、**プロファイル単位**か、両方か。両方が理想だが、セッション全体（`connManager` にフラグ1つ）の方が配線が半分で済む。まずセッション全体だけ実装し、プロファイル単位は必要になってから足すのが VISION の「複雑化するなら実装しない」に沿う。
2. `profiles.yaml` にキーを増やすかどうか（1と連動）。
3. 環境変数 `ASQL_READONLY=1` を用意するか。CI やラッパスクリプトからは便利だが、フラグだけで足りるなら足さない。
