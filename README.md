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

辞書を手動で更新したい場合は、手順2を再度実行してサービスを再起動してください：

```bash
yaskkserv2_make_dictionary \
  --dictionary-filename=$(brew --prefix)/var/yaskkserv2/dictionary.yaskkserv2 \
  SKK-JISYO.L

brew services restart yaskkserv2
```

## 辞書の自動更新

複数の上流 SKK 辞書を取得して `dictionary.yaskkserv2` を再構築する処理を自動化する
スクリプトを同梱しています。AquaSKK が HTTP で行っていた辞書の自動更新を置き換えるものです。

各 Mac が上流を pull してローカルで再構築し (源辞書は git に持ちません)、変化があったときだけ
辞書を原子的に差し替えてサービスを再起動します。

### セットアップ

```bash
./bin/setup
```

これで以下が行われます (冪等):

- 源辞書キャッシュ `$(brew --prefix)/var/yaskkserv2/sources/` の作成
- LaunchAgent (`com.delphinus.yaskkserv2-dict`) を `~/Library/LaunchAgents/` にシンボリックリンクして登録 (日次 04:23 / スリープ中に逃した分は起床時に実行)
- LaunchAgent (`com.delphinus.yaskkserv2-watchdog`) を登録 (60 秒ごとに死活監視。下記参照)
- 初回ビルド (`bin/build-dict --force`)

ログは `~/Library/Logs/yaskkserv2-dict.log` に出力されます。

### 手動実行 / 停止

```bash
# 今すぐ再構築 (変化が無くても強制)
./bin/build-dict --force

# 自動更新を止める
launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.delphinus.yaskkserv2-dict.plist
```

### 取り込む辞書

`bin/build-dict` 冒頭の `GIT_SOURCES` / `JISYO_FILES` で管理しています。
現在の取得元は以下のとおりです (`yaskkserv2_make_dictionary` は EUC-JP / UTF-8 をファイル単位で
自動判別するため、エンコーディングの変換は不要です)。

| 取得元 | 辞書 |
| --- | --- |
| [skk-dev/dict](https://github.com/skk-dev/dict) | L, jinmei, geo, propernoun, station, law, okinawa, assoc, edict, fullname, itaiji, itaiji.JIS3_4, JIS2, JIS3_4, JIS2004, mazegaki, china_taiwan, zipcode, office.zipcode |
| [uasi/skk-emoji-jisyo](https://github.com/uasi/skk-emoji-jisyo) | emoji |
| [ymrl/SKK-JISYO.emoji-ja](https://github.com/ymrl/SKK-JISYO.emoji-ja) | emoji-ja |
| [KeenS/SKK_JISYO.wiktionary](https://github.com/KeenS/SKK_JISYO.wiktionary) | shikakugoma |
| [tokuhirom/jawiki-kana-kanji-dict](https://github.com/tokuhirom/jawiki-kana-kanji-dict) | jawiki (GitHub Releases から取得) |

辞書を追加・削除したい場合は `bin/build-dict` の該当配列を編集してください。

## 死活監視 (watchdog)

yaskkserv2 0.1.7 はシングルスレッドのブロッキング TCP サーバで、macOS のスリープ/復帰で
ソケット syscall にハマると、**プロセスは生きていてポートも LISTEN しているのに、accept や
応答だけが止まる** ことがあります。この「生きたまま hang」した状態は launchd の `KeepAlive`
(プロセスが終了したときだけ再起動する) では復旧できず、SKK クライアント (skkeleton / Neovim
など) が応答待ちのままフリーズします。

`bin/yaskkserv2-watchdog` は SKK プロトコルで実際に `localhost:1178` へ問い合わせ、無応答なら
プロセスを入れ替えて復旧します。`bin/setup` が LaunchAgent
(`com.delphinus.yaskkserv2-watchdog`) として 60 秒ごとに実行するよう登録します。

```bash
# 今の状態を確認するだけ (復旧はしない。健全=0 / 異常=1 を返す)
./bin/yaskkserv2-watchdog --check

# 応答しなければその場で復旧する
./bin/yaskkserv2-watchdog

# 監視を止める
launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.delphinus.yaskkserv2-watchdog.plist
```

ログは `~/Library/Logs/yaskkserv2-watchdog.log` に出力されます (健全なときは何も書きません)。

### 補助: `--max-connections` の引き上げ

hang の引き金は、デフォルトの接続上限 `--max-connections=16` が低いことです。yaskkserv2 には
接続/アイドルタイムアウトが無いため、スリープ/復帰で切れた接続のスロットが解放されずに溜まり、
16 個埋まると新規接続を accept 直後に切る (= 無応答) 状態に陥ります。Formula の `service` ブロックで
上限を `1024` に引き上げ、飽和までの猶予を稼いでいます (watchdog が主防御、これは補助)。

既存環境へ反映する手順。`brew services` は **インストール済み keg の formula 受信コピー**
(`<Cellar>/yaskkserv2/<version>/.brew/yaskkserv2.rb`) の `service` ブロックから plist を生成する
ため、tap を更新して `brew services restart` するだけでは反映されません。keg を作り直す
`brew reinstall` が必要です:

```bash
brew update
brew reinstall yaskkserv2          # keg の formula 受信コピーを更新 (HEAD は再ビルド)
brew services restart yaskkserv2   # 新しい引数で plist を再生成・再起動
```

未インストールの環境では `brew install --HEAD yaskkserv2` で最初から反映されます。

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
