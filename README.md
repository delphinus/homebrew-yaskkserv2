# homebrew-yaskkserv2

[yaskkserv2](https://github.com/wachikun/yaskkserv2) の Homebrew Tap です。

## インストール

### Tap の追加

```bash
brew tap delphinus/yaskkserv2
```

### インストール

最新の開発版（推奨）:

```bash
brew install --HEAD yaskkserv2
```

安定版（0.1.7、2023年9月リリース）:

```bash
brew install yaskkserv2
```

## 使い方

### 1. SKK 辞書のダウンロード

まず SKK 辞書をダウンロードします。例えば：

```bash
# SKK 辞書をダウンロード
curl -O https://raw.githubusercontent.com/skk-dev/dict/master/SKK-JISYO.L

# または複数の辞書を使用
curl -O https://raw.githubusercontent.com/skk-dev/dict/master/SKK-JISYO.jinmei
curl -O https://raw.githubusercontent.com/skk-dev/dict/master/SKK-JISYO.geo
```

### 2. yaskkserv2 用の辞書を作成

```bash
yaskkserv2_make_dictionary \
  --dictionary-filename=$(brew --prefix)/var/yaskkserv2/dictionary.yaskkserv2 \
  SKK-JISYO.L
```

複数の辞書をマージすることもできます：

```bash
yaskkserv2_make_dictionary \
  --dictionary-filename=$(brew --prefix)/var/yaskkserv2/dictionary.yaskkserv2 \
  SKK-JISYO.L SKK-JISYO.jinmei SKK-JISYO.geo
```

### 3. サービスの起動

```bash
# サービスとして起動（ログイン時に自動起動）
brew services start yaskkserv2

# または手動で起動
yaskkserv2 $(brew --prefix)/var/yaskkserv2/dictionary.yaskkserv2
```

### 4. SKK クライアントの設定

Emacs の ddskk などの SKK クライアントから `localhost:1178` で接続できます。

#### Emacs (ddskk) の設定例

```elisp
(setq skk-server-host "localhost")
(setq skk-server-portnum 1178)
```

## サービスの管理

```bash
# 起動
brew services start yaskkserv2

# 停止
brew services stop yaskkserv2

# 再起動
brew services restart yaskkserv2

# 状態確認
brew services list
```

## ログの確認

```bash
tail -f $(brew --prefix)/var/log/yaskkserv2.log
```

## 辞書の更新

辞書を更新したい場合は、手順2を再度実行してサービスを再起動してください：

```bash
yaskkserv2_make_dictionary \
  --dictionary-filename=$(brew --prefix)/var/yaskkserv2/dictionary.yaskkserv2 \
  SKK-JISYO.L

brew services restart yaskkserv2
```

## Google 日本語入力との連携

yaskkserv2 は Google 日本語入力 API を使用して、辞書にない単語を変換することができます。

```bash
yaskkserv2 \
  --google-japanese-input=notfound \
  $(brew --prefix)/var/yaskkserv2/dictionary.yaskkserv2
```

詳しいオプションは `yaskkserv2 --help` を参照してください。

## アンインストール

```bash
# サービスの停止
brew services stop yaskkserv2

# アンインストール
brew uninstall yaskkserv2

# 辞書ファイルとログを削除する場合
rm -rf $(brew --prefix)/var/yaskkserv2
rm -f $(brew --prefix)/var/log/yaskkserv2.log
```

## ライセンス

yaskkserv2 は Apache License 2.0 または MIT License のデュアルライセンスです。
