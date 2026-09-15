class Yaskkserv2 < Formula
  desc "Rust で書かれた高速な SKK 辞書サーバー"
  homepage "https://github.com/wachikun/yaskkserv2"
  # リリース版 (0.1.7, 2023年9月) は古いため、master の特定コミットに固定して配布する。
  # コミットを進めるときは url の sha・sha256・pinned commit コメントを更新し、revision を
  # +1 する (upstream の Cargo バージョンは 0.1.7 のままなので、revision を上げないと
  # brew upgrade がコミット差し替えを検知しない)。service ブロックだけ変えた場合も同様。
  # pinned commit: f5bc4590c798c591e9861e02ea2e12d227a047ed (2026-07-01 時点の master HEAD)
  url "https://github.com/wachikun/yaskkserv2/archive/f5bc4590c798c591e9861e02ea2e12d227a047ed.tar.gz"
  version "0.1.7"
  sha256 "942525683f6725475468af42d9387b650e443f207e00d10a909eb86110fbe946"
  license any_of: ["Apache-2.0", "MIT"]
  revision 2
  head "https://github.com/wachikun/yaskkserv2.git", branch: "master"

  depends_on "rust" => :build

  # 送りあり見出しを Google 日本語入力へ問い合わせないようにする。upstream へ提案予定。
  # 詳細は下の __END__ 以降のコメントを参照。
  patch :DATA

  def install
    system "cargo", "install", *std_cargo_args
  end

  post_install_steps do
    # 辞書ディレクトリの作成
    mkdir_p "yaskkserv2"
  end

  service do
    # --max-connections のデフォルトは 16 と小さい。yaskkserv2 には接続/アイドル
    # タイムアウトが無いため、スリープ/復帰で切れた接続のスロットが解放されずに
    # 溜まり、16 個埋まると新規接続を accept 直後に切る (= 無応答ハング) 状態に陥る。
    # 上限を上げて飽和までの猶予を稼ぐ (根治は bin/yaskkserv2-watchdog 側で行う)。
    # --no-daemonize: yaskkserv2 はデフォルトで自己デーモン化する。launchd 配下では
    # fork されると launchd が直接の子の終了を見て「停止」と判断し、fork された実体が
    # orphan として port を握り続ける (KeepAlive が効かず、再起動時に古い orphan が
    # port を握ったまま新インスタンスが bind できない)。フォアグラウンドで動かして
    # launchd に PID を追跡させる。
    # --midashi-utf8: 見出しを UTF-8 で受け取る。クライアント側も見出しを UTF-8 で
    # 送る設定にしておくこと。
    # --google-cache-filename: 辞書に無い読みを Google 日本語入力へフォールバック
    # (--google-japanese-input=notfound, デフォルト) した結果をキャッシュし、2 回目
    # 以降の同じ読みはネット往復を省く。単一インスタンス運用なので排他制御は不要。
    # 他の --google-* はデフォルトのまま (suggest off / HTTPS / timeout 1000ms など)。
    run [
      opt_bin/"yaskkserv2",
      "--no-daemonize",
      "--max-connections=1024",
      "--midashi-utf8",
      "--google-cache-filename=#{var}/yaskkserv2/google.cache",
      var/"yaskkserv2/dictionary.yaskkserv2",
    ]
    keep_alive true
    log_path var/"log/yaskkserv2.log"
    error_log_path var/"log/yaskkserv2.log"
  end

  def caveats
    <<~EOS
      yaskkserv2 を使用するには、まず辞書ファイルを作成する必要があります。

      1. SKK 辞書をダウンロード:
         例: https://skk-dev.github.io/dict/ から SKK-JISYO.L をダウンロード

      2. yaskkserv2 用の辞書を作成:
         yaskkserv2_make_dictionary \\
           --dictionary-filename=#{var}/yaskkserv2/dictionary.yaskkserv2 \\
           /path/to/SKK-JISYO.L

      3. サービスを起動:
         brew services start yaskkserv2

      辞書ファイルは #{var}/yaskkserv2/ に配置されます。
      ログは #{var}/log/yaskkserv2.log に出力されます。

      SKK クライアント（Emacs の ddskk など）からは localhost:1178 で接続できます。

      注意: この Formula の stable は upstream master の特定コミットに固定されています
      (リリース版 0.1.7 は 2023年9月と古いため)。常に最新の master を追いたい場合は
      HEAD インストールも利用できます:
         brew install --HEAD yaskkserv2
    EOS
  end

  test do
    system bin/"yaskkserv2", "--version"
    system bin/"yaskkserv2_make_dictionary", "--help"
  end
end

__END__
--- a/src/skk/yaskkserv2/dictionary_reader.rs
+++ b/src/skk/yaskkserv2/dictionary_reader.rs
@@ -43,8 +43,15 @@
         let mut result = Vec::with_capacity(RESULT_VEC_CAPACITY);
         result.push(b'1');
         let midashi = Self::get_midashi(midashi_buffer);
+        // 送りあり見出し ("かんがえk" など) は Google Japanese Input へ問い合わせない。
+        // Google は送り仮名を表す末尾のアルファベットを解釈できず、仮名部分に対する
+        // 候補 (= 送りなし候補) を返すため、それをそのまま送りあり候補として配ると
+        // 誤った変換結果になる ("かんがえk" に対し "考え" や "カンガエ" が返る)。
+        // 見出しを機械的に生成して問い合わせるクライアント (補完など) では、辞書に
+        // 無い見出しが大量に生成されるため、この誤りと待ち時間が入力のたびに発生する。
+        let is_okuri_ari = Self::is_okuri_ari(midashi);
         let dictionary_midashi_key = Dictionary::get_dictionary_midashi_key(&midashi_buffer[1..])?;
-        if self.config.google_timing == GoogleTiming::First {
+        if !is_okuri_ari && self.config.google_timing == GoogleTiming::First {
             // Google API など、外部要因エラーは無視して継続させることに注意
             let _ignore_error_and_continue = self.read_google_candidates(midashi, &mut result);
         }
@@ -59,9 +66,10 @@
                 &mut result,
             )?;
         }
-        if self.config.google_timing == GoogleTiming::Last
-            || (self.config.google_timing == GoogleTiming::NotFound
-                && Yaskkserv2::is_empty_candidates(&result))
+        if !is_okuri_ari
+            && (self.config.google_timing == GoogleTiming::Last
+                || (self.config.google_timing == GoogleTiming::NotFound
+                    && Yaskkserv2::is_empty_candidates(&result)))
         {
             let _ignore_error_and_continue = self.read_google_candidates(midashi, &mut result);
         }
