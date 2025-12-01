# Claude Code ガイド

このドキュメントは、Claude Code を使用してこのリポジトリに貢献する際のガイドラインです。

## コミットメッセージ規約

このリポジトリでは、[Conventional Commits](https://www.conventionalcommits.org/) に従ったコミットメッセージを使用します。

### 基本形式

```
<type>: <subject>

<body>

<footer>
```

### Type の種類

- `feat`: 新機能の追加
- `fix`: バグ修正
- `docs`: ドキュメントのみの変更
- `style`: コードの意味に影響しない変更（フォーマット、セミコロンの追加など）
- `refactor`: リファクタリング（バグ修正や機能追加を含まない）
- `perf`: パフォーマンス改善
- `test`: テストの追加や修正
- `chore`: ビルドプロセスやツールの変更

### 例

```
feat: yaskkserv2 の Homebrew Formula を追加

- yaskkserv2.rb: yaskkserv2 の Formula（HEAD インストール推奨）
- README.md: インストール手順と使い方のドキュメント

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

```
fix: ライセンス表記を any_of 形式に修正

brew audit でのライセンスエラーを修正。
デュアルライセンスは any_of 形式で記述する必要がある。

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

## 言語

このリポジトリは日本語話者向けのため、以下のように日本語を使用します：

- コミットメッセージ: 日本語
- ドキュメント: 日本語
- Formula の description や caveats: 日本語

## Homebrew Formula のガイドライン

### HEAD インストールの推奨

yaskkserv2 の最新リリース（0.1.7）は 2023年9月と古いため、HEAD インストールを推奨します。

### テスト

Formula の変更は、GitHub Actions で自動的にテストされます：

- yaskkserv2 HEAD のインストール
- インストールの検証（バイナリの存在確認）
- `brew test` の実行
- `brew audit --strict` の実行

全てのテストが成功しない限り、main ブランチへのマージはできません。

## ブランチ戦略

- `main`: 安定版。直接プッシュ禁止。
- feature ブランチ: 新機能や修正は feature ブランチで作業し、PR を作成してマージ。

## PR のマージ条件

- 全ての GitHub Actions テストが成功していること
- Branch Protection Rule により、テストが成功しない限りマージできません
