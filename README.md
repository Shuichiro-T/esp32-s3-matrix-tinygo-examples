# ESP32-S3-Matrix TinyGo Examples

[Waveshare ESP32-S3-Matrix](https://docs.waveshare.com/ESP32-S3-Matrix) を [TinyGo](https://tinygo.org/) で動かすサンプル集です。

## ボード概要

| 項目 | 内容 |
| --- | --- |
| SoC | ESP32-S3（Xtensa LX7 デュアルコア、最大 240MHz、Wi-Fi / Bluetooth LE） |
| メモリ | SRAM 512KB、Flash 4MB |
| LED | 8x8 = 64 個のアドレス指定可能な RGB LED（WS2812 互換） |
| IMU | QMI8658（3 軸加速度 + 3 軸ジャイロ） |
| USB | USB Type-C（ESP32-S3 内蔵の USB Serial/JTAG） |
| ボタン | BOOT、RESET |
| 電源 | LDO ME6217C33M5G（最大 800mA） |

### ピン配置

ピン番号は Waveshare 公式デモ（[ESP32-S3-Matrix-Demo.zip](https://files.waveshare.com/wiki/ESP32-S3-Matrix/ESP32-S3-Matrix-Demo.zip)）に基づいています。[board/board.go](board/board.go) にまとめてあります。

| 機能 | GPIO |
| --- | --- |
| RGB LED マトリクスのデータ線 | GPIO14 |
| IMU (QMI8658) I2C SDA | GPIO11 |
| IMU (QMI8658) I2C SCL | GPIO12 |
| QMI8658 の I2C アドレス | 0x6B |

その他の GPIO は[公式ドキュメント](https://docs.waveshare.com/ESP32-S3-Matrix)のピン配置図と[回路図](https://files.waveshare.com/wiki/ESP32-S3-Matrix/ESP32-S3-Matrix-Sch.pdf)を参照してください。

## ⚠️ LED の輝度に関する注意

Waveshare は「**LED の輝度を上げすぎると基板の温度が急上昇し、基板を破損するおそれがある**」と注意しています。公式デモでも輝度は 40〜60/255 程度です。

このリポジトリの `matrix` パッケージは、輝度を `board.MaxBrightness`（64/255）までに制限しています。上限を変える場合は、発熱に十分注意してください。

USB 給電の電圧が約 4.9V を下回ると、USB 接続が切れたりリセットを繰り返したりすることがあります（公式 FAQ より）。

## 必要なもの

- [Go](https://go.dev/dl/) 1.25 以降
- [TinyGo](https://tinygo.org/getting-started/install/) 0.41 以降（動作確認: 0.41.1、0.42.0）
  - `06_ble_color` と `07_music_ripple`（Bluetooth）は **TinyGo 0.42 以降**が必要です
- USB Type-C ケーブル（データ通信に対応したもの）

macOS の場合:

```sh
brew tap tinygo-org/tools
brew install tinygo

# 0.41 からアップデートする場合
brew upgrade tinygo
```

TinyGo のターゲットには `esp32s3-generic` を使います。

## 使い方

### ビルドと書き込み

```sh
# 依存モジュールの取得
go mod download

# 書き込みポートを確認
tinygo ports

# 書き込み（終わったらそのままシリアルモニタを開く）
tinygo flash -target esp32s3-generic -monitor ./examples/01_color_check

# ポートを指定する場合
tinygo flash -target esp32s3-generic -port /dev/cu.usbmodem1101 ./examples/02_rainbow
```

Makefile も用意しています。

```sh
make flash EXAMPLE=02_rainbow             # 書き込み + モニタ
make flash EXAMPLE=02_rainbow PORT=/dev/cu.usbmodem1101
make build EXAMPLE=02_rainbow             # build/02_rainbow.bin を生成
make build-all                            # 全サンプルをビルド
make monitor                              # シリアルモニタのみ
```

### 書き込みできないとき（ダウンロードモード）

書き込みに失敗する、またはポートが見つからない場合は、手動でダウンロードモードに入れてください。

1. **BOOT** ボタンを押したままにする
2. **RESET** ボタンを押して離す
3. **BOOT** ボタンを離す

書き込みが終わったら RESET を押してプログラムを起動します。

## サンプル一覧

| ディレクトリ | 内容 |
| --- | --- |
| [examples/01_color_check](examples/01_color_check/main.go) | 全面を赤 → 緑 → 青 → 白の順に点灯し、左上から 1 ピクセルずつ走査する。色の順序と座標の向きを確認する |
| [examples/02_rainbow](examples/02_rainbow/main.go) | 斜め方向に流れる虹色アニメーション |
| [examples/03_imu_serial](examples/03_imu_serial/main.go) | QMI8658 の加速度・角速度・温度を USB シリアルに出力する |
| [examples/04_tilt_dot](examples/04_tilt_dot/main.go) | 基板を傾けると光の点が転がる（公式デモの "Game" に相当） |
| [examples/05_scroll_text](examples/05_scroll_text/main.go) | [tinyfont](https://github.com/tinygo-org/tinyfont) を使って文字列をスクロール表示する |
| [examples/06_ble_color](examples/06_ble_color/main.go) | Bluetooth LE で Chrome と接続し、ブラウザから LED の色と輝度を変更する（TinyGo 0.42 以降） |
| [examples/07_music_ripple](examples/07_music_ripple/main.go) | ブラウザで再生した YouTube の音に合わせて、低音のビートで四角い波紋、音の出だしで星を光らせる（TinyGo 0.42 以降） |

最初に `01_color_check` を書き込んで、LED の色の順序と表示の向きを確認してください。

## 06_ble_color: ブラウザから Bluetooth で色を変える

ESP32-S3 内蔵の Bluetooth LE を [tinygo.org/x/bluetooth](https://github.com/tinygo-org/bluetooth) で使い、GATT サーバとして動かします。ブラウザ側は Chrome の [Web Bluetooth API](https://developer.mozilla.org/ja/docs/Web/API/Web_Bluetooth_API) を使った 1 枚の HTML（[web/index.html](examples/06_ble_color/web/index.html)）です。

### 手順

1. ボードに書き込む（TinyGo 0.42 以降）

   ```sh
   make flash EXAMPLE=06_ble_color
   ```

   起動するとマトリクスが暗い白で点灯し、BLE の準備ができると青に変わります。デバイス名 `S3-Matrix` でアドバタイズを始めます。

2. ページを配信する

   ```sh
   make serve EXAMPLE=06_ble_color   # python3 -m http.server で http://localhost:8080 を配信
   ```

   Web Bluetooth は安全なコンテキストでしか使えません。`http://localhost` は安全とみなされるので、開発中は HTTPS がなくても動きます。

3. Chrome で <http://localhost:8080> を開き、「接続する」を押して `S3-Matrix` を選ぶ

   カラーピッカー・プリセット・輝度スライダーを操作すると、その場で LED に反映されます。

macOS では「システム設定 → プライバシーとセキュリティ → Bluetooth」で Chrome を許可しておく必要があります。Web Bluetooth は Chrome / Edge（デスクトップ・Android）で使えます。Safari、Firefox、iOS のブラウザは対応していません。

### GATT の構成

| 項目 | UUID | 内容 |
| --- | --- | --- |
| サービス | `6e400001-7a3c-4f1b-9d2e-5c8b1f0a3e01` | |
| 色 | `6e400002-7a3c-4f1b-9d2e-5c8b1f0a3e01` | 3 バイト（R, G, B）。Read / Write |
| 輝度 | `6e400003-7a3c-4f1b-9d2e-5c8b1f0a3e01` | 1 バイト（0〜255、`board.MaxBrightness` で切り詰め）。Read / Write |

UUID を変える場合は [main.go](examples/06_ble_color/main.go) と [index.html](examples/06_ble_color/web/index.html) の両方を書き換えてください。

### 実装上の工夫

- BLE の書き込みコールバックでは LED を更新せず、チャネルでメインループに値を渡します。LED の送信中は割り込みを禁止するため、BLE スタックの処理の中で行わないようにしています。
- `tinygo.org/x/bluetooth` v0.17.0 の HCI 実装（ESP32 で使われるもの）は、Write Without Response を受け取っても無視します。そのためブラウザ側は `writeValueWithResponse` で書き込みます。
- 同じ実装では、ボード側で `Characteristic.Write()` を呼ぶと自分の `WriteEvent` も呼ばれます。読み出し値を更新するときは、`Value` に渡したスライスを直接書き換えています。
- 同じ実装では、アドバタイズに 128 ビットのサービス UUID を載せられません。そのためブラウザ側は、デバイス名 `S3-Matrix` で探して、サービスへのアクセスは `optionalServices` で許可しています（07 も同じ）。
- スライダーを動かすと書き込みが大量に届きます。そこで、ボード側もブラウザ側も「最新の値だけ」を残し、古い値は捨てます。

## 07_music_ripple: YouTube の音に合わせて光らせる

ブラウザで YouTube の動画を再生し、その音を解析して LED を光らせます。

- **低音のビート**：中心から四角い波紋が、2x2 → 4x4 → 6x6 → 8x8 の順に外へ広がる
- **音の出だし（オンセット）**：ランダムな 1 点が星のように光って消える
- 色相は時間とともに少しずつ虹色に変わる

8x8 では円の波紋はぼやけて見えるので、ピクセルの格子に沿った四角い輪（中心からのチェビシェフ距離で 4 段階）にしています。ボードとの接続は 06 と同じく Web Bluetooth です。

### 仕組み

埋め込んだ YouTube（iframe）は別オリジンなので、ページから音声データを直接は読めません。そこで、Chrome の画面共有 API（`getDisplayMedia`）で**自分のタブの音声**を取り込み、Web Audio API の `AnalyserNode` で解析します。

```text
[Chrome]  YouTube iframe ──タブ音声の取り込み──▶ AnalyserNode (FFT)
                                                  │ 低音 40〜150Hz のエネルギー
                                                  │ 40Hz〜5kHz の spectral flux
                                                  ▼
                                    低音ビート / オンセット検出
                                                  │ BLE で 4 バイト
[ボード]  ◀───────────────────────────────────────┘
          番号が変わったら波紋や星を出す。アニメーションはボード側で約 40fps で描く
```

ビートは 2 種類の方法で検出します。

- **低音ビート**：低音のエネルギーが、最近の平均より大きく跳ね上がったとき。キックドラムのはっきりした曲向けです。
- **オンセット（音の出だし）**：40Hz〜5kHz で前のフレームより強くなった成分の合計（spectral flux）が、直近約 0.7 秒の「平均 + k × 標準偏差」を超えたとき。ピアノ曲やバラードなど、ビートの分かりにくい曲向けの補助です。直近 1.2 秒に低音ビートがあればこちらは使わず、波紋も少し弱めにします。

ビートの判定はブラウザで、描画はボードで行います。BLE の書き込みは応答待ちが必要で、毎秒 20〜30 回程度が上限の見込みです。この分担にしておくと、通信の間隔が空いてもアニメーションは滑らかに動きます。ビートとオンセットはそれぞれ「番号」で送るので、途中の書き込みが最新値で上書きされても、ビートを取りこぼしません。

### 手順

1. ボードに書き込む（TinyGo 0.42 以降）

   ```sh
   make flash EXAMPLE=07_music_ripple
   ```

   BLE の準備ができると、中心から弱い波紋がゆっくり出る待機表示になります。

2. ページを配信する

   ```sh
   make serve EXAMPLE=07_music_ripple
   ```

3. Chrome で <http://localhost:8080> を開き、次の順に操作する
   1. 「接続する」を押して `S3-Matrix` を選ぶ
   2. YouTube の URL（または動画 ID）を入力して「読み込む」を押し、再生する
   3. 「このタブの音声を取り込む」を押す。共有ダイアログで**このタブ**を選び、**「タブの音声も共有する」をオン**にして共有する

   ページのメーターが低音に合わせて動き、ビートで丸が光れば解析できています（低音ビートは青、オンセットは紫）。ビートを拾いすぎる、または拾わない場合は「ビートの感度」を調整してください。

### 注意

- 使えるのは Chrome / Edge のデスクトップ版だけです（タブ音声の取り込みに対応していないブラウザがあるため）。
- 埋め込み再生が許可されていない動画は再生できません。
- タブを裏に回すと `requestAnimationFrame` が止まり、解析も止まります。このタブを表示したままにしてください。
- 音の情報が 1.5 秒以上届かないと、ボードは待機表示に戻ります（一時停止したときなど）。
- 波紋の速さ・残像の長さ、星の消え方、色相が変わる速さは [main.go](examples/07_music_ripple/main.go) 冒頭の定数で調整できます。

### GATT の構成

| 項目 | UUID | 内容 |
| --- | --- | --- |
| サービス | `6e400011-7a3c-4f1b-9d2e-5c8b1f0a3e01` | 06 とは別の UUID（06 のページで接続しても、サービスが見つからずエラーになる） |
| 音の情報 | `6e400012-7a3c-4f1b-9d2e-5c8b1f0a3e01` | 4 バイト（低音レベル 0〜255、低音ビート番号、オンセット番号、強さ 0〜255）。Write |
| 輝度 | `6e400013-7a3c-4f1b-9d2e-5c8b1f0a3e01` | 1 バイト（06 と同じ）。Read / Write |

## リポジトリ構成

```text
.
├── board/      ピン番号などボード固有の定数
├── matrix/     8x8 LED のフレームバッファ（drivers.Displayer を実装）
├── ws2812s3/   ESP32-S3 用 WS2812 ドライバ（ビットバンギング）
├── qmi8658/    QMI8658 IMU の最小限のドライバ
└── examples/   サンプルプログラム（06・07 の web/ はブラウザ側の HTML）
```

### `matrix` パッケージ

```go
m, err := matrix.New()          // GPIO14 の LED マトリクスを初期化
m.SetBrightness(20)             // 輝度 0〜255（上限 board.MaxBrightness）
m.SetPixel(x, y, color.RGBA{R: 255, A: 255}) // 原点 (0,0) は左上
m.Fill(c) / m.Clear()
m.Display()                     // フレームバッファを LED に送信

m.Order = matrix.GRB            // 色の順序（既定値は RGB）
m.Rotation = matrix.Rotate90    // 表示を回転
```

`matrix.Matrix` は `tinygo.org/x/drivers.Displayer` を実装しています。そのため [tinyfont](https://github.com/tinygo-org/tinyfont) や [tinydraw](https://github.com/tinygo-org/tinydraw) の描画先にそのまま渡せます。

### 色の順序と向きについて

公式デモのコードでは、LED の色の順序（`NEO_RGB` と `NEO_GRB`）とマトリクスの並び方がサンプルごとに異なります。このリポジトリでは、虹色表示を行う公式 "Color" デモに合わせて、既定値を **RGB 順・行優先（左上から右へ）** にしています。

- 赤と緑が入れ替わって表示される場合は `m.Order = matrix.GRB` を設定してください。
- 表示の向きが持ち方と合わない場合は `m.Rotation` で回転させてください。

## 実装メモ

### WS2812 ドライバを自前で実装している理由

TinyGo 0.41.1 と `tinygo.org/x/drivers` v0.36.0〜v0.37.0 の組み合わせでは、`ws2812` パッケージを ESP32-S3 向けにビルドできません。Xtensa 向けの実装が、ESP32-S3 の `machine` パッケージに存在しない `machine.CPUFrequency()` を参照しているためです。

そこで [ws2812s3](ws2812s3/ws2812s3.go) では、Xtensa の CPU サイクルカウンタ（`CCOUNT`）で時間を計りながら GPIO を操作しています。

- 送信中（64 LED で約 2ms）は割り込みを禁止します。
- プログラムはフラッシュ上でキャッシュを通して実行されます。最初のビットがキャッシュミスで遅れないよう、本送信の前にピンを動かさない空送信を一度行い、キャッシュを温めています。

将来、公式ドライバが ESP32-S3 に対応したら、そちらへの置き換えを検討してください。

### Bluetooth について

`tinygo.org/x/bluetooth` は v0.17.0 から、[espradio](https://github.com/tinygo-org/espradio) を通して ESP32-S3 内蔵の BLE 無線を直接使えるようになりました。TinyGo 0.42 以降はターゲットに `espradio` ビルドタグを自動で付けます。TinyGo 0.41 では、タグを手動で付けても無線のバイナリブロブのリンクに失敗します。

### QMI8658 ドライバについて

`tinygo.org/x/drivers/qmi8658c` もありますが、v0.36.0〜v0.37.0 には加速度・角速度の下位バイトを読む位置がずれている不具合があります。そのため [qmi8658](qmi8658/qmi8658.go) に、必要な機能だけを実装しました。設定は加速度 ±4g、ジャイロ ±512dps、約 224Hz です。

### エディタ（VS Code / gopls）の設定

`machine` パッケージは TinyGo にしかないため、通常の `go vet` や gopls ではエラーになります。VS Code では [TinyGo 拡張機能](https://marketplace.visualstudio.com/items?itemName=tinygo.vscode-tinygo)を入れ、コマンドパレットの「TinyGo target」で `esp32s3-generic` を選んでください。

## 参考資料

- [ESP32-S3-Matrix 公式ドキュメント](https://docs.waveshare.com/ESP32-S3-Matrix)
- [回路図 (PDF)](https://files.waveshare.com/wiki/ESP32-S3-Matrix/ESP32-S3-Matrix-Sch.pdf)
- [公式デモコード (Arduino)](https://files.waveshare.com/wiki/ESP32-S3-Matrix/ESP32-S3-Matrix-Demo.zip)
- [ESP32-S3 データシート](https://files.waveshare.com/wiki/common/Esp32-s3_datasheet_en.pdf)
- [ESP32-S3 テクニカルリファレンスマニュアル](https://files.waveshare.com/wiki/common/Esp32-s3_technical_reference_manual_en.pdf)
- [TinyGo ドキュメント](https://tinygo.org/docs/)
