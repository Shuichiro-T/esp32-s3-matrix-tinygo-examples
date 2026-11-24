# 06_ble_color: ブラウザから Bluetooth で色を変える

[← リポジトリの README に戻る](../../README.md)（ビルド環境の準備、書き込み方法など）

ESP32-S3 内蔵の Bluetooth LE を [tinygo.org/x/bluetooth](https://github.com/tinygo-org/bluetooth) で使い、GATT サーバとして動かします。ブラウザ側は Chrome の [Web Bluetooth API](https://developer.mozilla.org/ja/docs/Web/API/Web_Bluetooth_API) を使った 1 枚の HTML（[web/index.html](web/index.html)）です。

## 手順

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

## GATT の構成

| 項目 | UUID | 内容 |
| --- | --- | --- |
| サービス | `6e400001-7a3c-4f1b-9d2e-5c8b1f0a3e01` | |
| 色 | `6e400002-7a3c-4f1b-9d2e-5c8b1f0a3e01` | 3 バイト（R, G, B）。Read / Write |
| 輝度 | `6e400003-7a3c-4f1b-9d2e-5c8b1f0a3e01` | 1 バイト（0〜255、`board.MaxBrightness` で切り詰め）。Read / Write |

UUID を変える場合は [main.go](main.go) と [index.html](web/index.html) の両方を書き換えてください。

## 実装上の工夫

- BLE の書き込みコールバックでは LED を更新せず、チャネルでメインループに値を渡します。LED の送信中は割り込みを禁止するため、BLE スタックの処理の中で行わないようにしています。
- `tinygo.org/x/bluetooth` v0.17.0 の HCI 実装（ESP32 で使われるもの）は、Write Without Response を受け取っても無視します。そのためブラウザ側は `writeValueWithResponse` で書き込みます。
- 同じ実装では、ボード側で `Characteristic.Write()` を呼ぶと自分の `WriteEvent` も呼ばれます。読み出し値を更新するときは、`Value` に渡したスライスを直接書き換えています。
- 同じ実装では、アドバタイズに 128 ビットのサービス UUID を載せられません。そのためブラウザ側は、デバイス名 `S3-Matrix` で探して、サービスへのアクセスは `optionalServices` で許可しています（[07_music_ripple](../07_music_ripple/README.md) も同じ）。
- スライダーを動かすと書き込みが大量に届きます。そこで、ボード側もブラウザ側も「最新の値だけ」を残し、古い値は捨てます。
