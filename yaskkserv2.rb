class Yaskkserv2 < Formula
  desc "Rust で書かれた高速な SKK 辞書サーバー"
  homepage "https://github.com/wachikun/yaskkserv2"
  url "https://github.com/wachikun/yaskkserv2/archive/refs/tags/0.1.7.tar.gz"
  sha256 "93831cd32cd60bf946fbfef988b85c288105134a9a3ac46cf82f0e18babd1d4b"
  license "Apache-2.0 OR MIT"
  head "https://github.com/wachikun/yaskkserv2.git", branch: "master"

  depends_on "rust" => :build

  def install
    system "cargo", "install", *std_cargo_args
  end

  def post_install
    # 辞書ディレクトリの作成
    (var/"yaskkserv2").mkpath
  end

  service do
    run [opt_bin/"yaskkserv2", var/"yaskkserv2/dictionary.yaskkserv2"]
    keep_alive true
    log_path var/"log/yaskkserv2.log"
    error_log_path var/"log/yaskkserv2.log"
  end

  test do
    system "#{bin}/yaskkserv2", "--version"
    system "#{bin}/yaskkserv2_make_dictionary", "--help"
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

      注意: リリース版 (0.1.7) は 2023年9月と古いため、最新機能を使いたい場合は
      HEAD インストールを推奨します:
         brew install --HEAD yaskkserv2
    EOS
  end
end
