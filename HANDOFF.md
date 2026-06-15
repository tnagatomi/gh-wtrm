# gh-wtrm 設計ハンドオフ

`tnagatomi/wtclean` を `tnagatomi/gh-wtrm`（gh CLI 拡張）として作り直すための設計仕様。
このドキュメント単体で新リポジトリでの実装に着手できることを目的とする。

## 1. 背景 — なぜ作り直すか

wtclean の `safe-to-remove` 判定にデータ損失リスクがある。

- `safe-to-remove` の肯定的証拠に **`upstream-gone`**（上流追跡ブランチが削除された＝`git for-each-ref` の `[gone]`）を含めている（`internal/tui/badges.go` の `safeToRemovePositive`）。
- `upstream-gone` は「マージ＋ブランチ自動削除」だけでなく、**PR を未マージでクローズしてブランチ削除**・手動削除・リモートでのリネームでも立つ。**マージされたことの証明ではない**。
- さらに上流が `[gone]` のとき git は ahead 数を計算できず **`unpushed` バッジが立たない**（`internal/repo/badges.go` の `ahead ` パースが `[gone]` 出力にマッチしない）。
- 結果、**リモートに存在しない未マージのローカルコミットを持つ worktree が safe-to-remove と判定され**、`s` 一括選択 → 削除（`git branch -D` 強制）で**未マージ作業が失われる**。

これを構造的に解消するため、ローカルヒューリスティック（`git branch --merged` + `upstream-gone`）を捨て、**GitHub の PR 実マージ状態**で判定する（gh-poi 方式）。

## 2. 確定した設計判断

| # | 決定 | 内容 |
|---|---|---|
| 1 | **gh 拡張として実装** | `gh-` プレフィックスの standalone バイナリ。`gh extension install` で導入、`gh wtrm` で起動。gh 必須依存が公理になる。認証・ホスト解決・API は `go-gh` 経由。 |
| 2 | **wtclean を置換** | wtclean は非推奨/凍結し gh-wtrm に一本化。オフライン動作・GitHub 以外のホスト対応は**捨てる**。 |
| 3 | **名前は `wtrm`** | リポジトリ `tnagatomi/gh-wtrm`、呼び出し `gh wtrm`。`wt`=worktree + `rm`=remove。`wtc` は WTC を想起するため不可。 |
| 4 | **単一リポジトリ専用** | 多数リポジトリ横断スキャンを廃止。CWD を含むリポジトリ1つを対象（≒ wtclean の `--cwd` モードが唯一のモード）。config / roots 探索 / リポジトリ一覧画面 / refresh を撤廃。 |
| 5 | **安全モデルは gh-poi と同一** | 下記 §3。`upstream-gone` を肯定的証拠から外し、PR 実マージ + コミット OID 一致で判定。 |
| 6 | **対話 TUI を維持** | wtclean の bubbletea TUI（チェックボックス選択・safe 一括選択・confirm 画面・also-delete-branch トグル）をそのまま活かす。「gh-poi の正確さ + wtclean の対話 worktree TUI」が差別化点。 |

## 3. 安全モデル（gh-poi のロジックを移植）

参照元: `github.com/seachicken/gh-poi`（ローカル: `../../seachicken/gh-poi`）

### 3.1 削除可否（`cmd/root.go` の `getDeleteStatus`, root.go:576-609）

ブランチが **Deletable** となるのは以下を**すべて**満たすとき:

1. ブランチが locked でない
2. worktree（ある場合）が以下のいずれでもない:
   - worktree が locked
   - メイン worktree かつ現在の HEAD でない
   - 非メイン worktree かつ現在の HEAD（＝**今いる worktree は消せない**）
   - 非メイン worktree かつ untracked ファイルあり
3. tracked な未コミット変更がない（`HasTrackedChanges`）
4. **PR が1つ以上存在する**（PR が無ければ削除不可。ローカル `git branch --merged` フォールバックは**しない** = 保守的）
5. Open な PR が1つも無い
6. 「fully merged」な PR が1つ以上ある

### 3.2 fully-merged 判定（`isFullyMerged`, root.go:611-628）— 核心

- ブランチに commit がある
- PR.State がスキャンモードと一致（**デフォルト `Merged` のみ**。`--state closed` で Closed も対象に opt-in）
- **ローカル HEAD の commit OID（`branch.Commits[0]`）が PR の commits に含まれる**

この最後のチェックが肝。**PR マージ後にローカルで追加コミットすると HEAD OID が PR の commits に無い → 削除不可**となり、未マージのローカル作業を確実に保護する。これが wtclean の upstream-gone バグの根治。

### 3.3 デフォルト挙動

- `--state` のデフォルトは `merged`（`main.go:89`）。`--state closed` は opt-in。
- よって「CLOSED 未マージ」という最も危険なケースはデフォルトで対象外。

## 4. リポジトリ構成（移植元 → 移植先）

### wtclean から残す
- `internal/worktree/worktree.go` — `git worktree list --porcelain` パース。ほぼそのまま。`Badge` 体系は §5 に従い再設計。
- `internal/tui/*` — worktree 画面、`confirm.go`、`selection*.go`、`filter.go`、`help.go`、`keys.go`、`copy`、`empty`。`badges.go` の `safe-to-remove` ロジックは §3 に差し替え。
- `internal/deleter/deleter.go` — worktree remove + branch delete。force/branch のセマンティクスを gh-poi 準拠に調整。
- `internal/cli/`（`resolve.go`, cwd 解決）— CWD → リポジトリ主 worktree の解決に流用。
- `.goreleaser.yaml` / GitHub Actions — gh 拡張のプリコンパイル配布に転用（§6）。

### wtclean から捨てる
- `internal/config/` — config 不要。
- `internal/scanner/` — roots 探索不要。
- リポジトリ一覧画面・`refresh` 関連の TUI コード。
- `internal/repo/badges.go` の `mergedBranches` / `branchTracking` / `upstream-gone` 判定。

### 新規に足す（gh-poi から移植）
- GitHub GraphQL での PR 取得。gh-poi の `conn/`（`connection.go` など）と `shared/querygen.go`（クエリ生成）、`shared/pull_request.go`（`PullRequest` 構造体）、`cmd/root.go` の `getPullRequests`/`toPullRequestState`（root.go:239, 750-810 付近）を参照。
- ローカルブランチ → PR のマッピング: ブランチのローカル commit 群を集め、それらの commit を含む PR を GraphQL で問い合わせる（gh-poi の方式）。
- `go-gh/v2` での認証・API クライアント。

## 5. バッジ / 状態モデルの再設計

wtclean の badge を gh-poi のフィールドへ対応付ける:

| wtclean badge | gh-wtrm での扱い |
|---|---|
| `primary` | 維持（メイン worktree、削除不可） |
| `merged`（ローカル `git branch --merged`） | **廃止**。PR 実マージ判定に置換 |
| `upstream-gone` | **廃止**（バグ源） |
| `uncommitted` | `HasTrackedChanges` 相当。維持（削除不可条件） |
| `unpushed` | PR コミット OID 一致チェックで吸収。表示用に残すかは要検討 |
| `locked` | 維持 |
| `no-dir` | 維持（prune 対象） |
| （新）`pr-merged` / `pr-open` / `pr-closed` | PR 状態を表示。`pr-merged` かつ §3.2 成立が safe-to-remove の肯定的証拠 |

`safe-to-remove` = §3 の Deletable 判定そのもの。

## 6. gh 拡張としての配布

- gh 拡張（Go・プリコンパイル）はリポジトリ直下に `main.go`、リリースに各 OS/arch バイナリ（命名 `gh-wtrm`）。
- `cli/gh-extension-precompile` アクション、または既存 goreleaser 設定を流用。
- 導入: `gh extension install tnagatomi/gh-wtrm` → `gh wtrm` で起動。
- module path は `github.com/tnagatomi/gh-wtrm`。

## 7. 残課題（新リポジトリで詰める）

- `unpushed` 相当のバッジ表示を残すか（OID 一致チェックで安全性は担保されるが、UX 情報として有用か）。
- PR が複数ある場合の表示（gh-poi は複数 PR を扱う）。
- fork からの PR / 複数リモートの扱い（gh-poi の remote 解決ロジック `shared/remote.go` 参照）。
- GraphQL レート制限・エラー時の UX（取得失敗時は「PR 不明」として削除不可側に倒す＝安全側）。
- `--state closed` 相当を TUI でどう露出するか（フラグ / キーバインド）。
- `--cwd` 廃止に伴う CLI フラグ整理。

## 8. ハンドオフ手順

1. `gh repo create tnagatomi/gh-wtrm`（**未実行**。ユーザーのゴーサイン後に実行）。
2. このドキュメントを新リポジトリにコピーして起点にする。
3. §4 に従い wtclean のコードを移植、§3/§5 で安全モデルを差し替え。
4. gh-poi の GraphQL/PR 取得を移植。
5. §6 で gh 拡張として配布設定。
