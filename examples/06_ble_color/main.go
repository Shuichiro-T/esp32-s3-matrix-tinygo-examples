// 06_ble_color: Bluetooth LE で Chrome（Web Bluetooth）と接続し、
// ブラウザから LED マトリクスの色を変更します。
//
// ブラウザ側のページは web/index.html です。
// 使い方は README の「06_ble_color」の項を参照してください。
package main

import (
	"image/color"
	"time"

	"tinygo.org/x/bluetooth"

	"esp32-s3-matrix-tinygo-examples/matrix"
)

// GATT の UUID。web/index.html と同じ値にしてください。
var (
	serviceUUID = mustParseUUID("6e400001-7a3c-4f1b-9d2e-5c8b1f0a3e01")
	// 色: 3 バイト (R, G, B)。読み書き可能。
	colorUUID = mustParseUUID("6e400002-7a3c-4f1b-9d2e-5c8b1f0a3e01")
	// 輝度: 1 バイト (0〜255)。上限は board.MaxBrightness で切り詰められる。
	brightnessUUID = mustParseUUID("6e400003-7a3c-4f1b-9d2e-5c8b1f0a3e01")
)

const deviceName = "S3-Matrix"

var adapter = bluetooth.DefaultAdapter

// BLE のコールバックからメインループへ値を渡すチャネル。
// コールバック内では LED の送信（割り込み禁止を伴う）を行わない。
var (
	colorCh      = make(chan [3]byte, 1)
	brightnessCh = make(chan uint8, 1)
)

func main() {
	time.Sleep(time.Second) // シリアルモニタの接続待ち

	m, err := matrix.New()
	if err != nil {
		halt("matrix init failed: " + err.Error())
	}
	m.SetBrightness(20)

	// これらのスライスは GATT の読み出し値としてそのまま使われる。
	cur := [3]byte{0, 0, 255} // 起動時は青
	brightnessValue := []byte{m.Brightness()}

	// 起動中の表示（暗い白）
	m.Fill(color.RGBA{R: 40, G: 40, B: 40, A: 255})
	m.Display()

	must("enable BLE stack", adapter.Enable())

	adapter.SetConnectHandler(func(device bluetooth.Device, connected bool) {
		if connected {
			println("connected:", device.Address.String())
		} else {
			println("disconnected:", device.Address.String())
		}
	})

	var colorChar, brightnessChar bluetooth.Characteristic
	must("add service", adapter.AddService(&bluetooth.Service{
		UUID: serviceUUID,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &colorChar,
				UUID:   colorUUID,
				Value:  cur[:],
				// Write Without Response は ESP32 の HCI 実装では無視されるため、
				// 応答ありの Write だけを受け付ける。
				Flags: bluetooth.CharacteristicReadPermission |
					bluetooth.CharacteristicWritePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					if offset != 0 || len(value) != 3 {
						return
					}
					sendLatest(colorCh, [3]byte{value[0], value[1], value[2]})
				},
			},
			{
				Handle: &brightnessChar,
				UUID:   brightnessUUID,
				Value:  brightnessValue,
				// Write Without Response は ESP32 の HCI 実装では無視されるため、
				// 応答ありの Write だけを受け付ける。
				Flags: bluetooth.CharacteristicReadPermission |
					bluetooth.CharacteristicWritePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					if offset != 0 || len(value) != 1 {
						return
					}
					sendLatest(brightnessCh, value[0])
				},
			},
		},
	}))

	adv := adapter.DefaultAdvertisement()
	must("configure advertisement", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    deviceName,
		ServiceUUIDs: []bluetooth.UUID{serviceUUID},
	}))
	must("start advertisement", adv.Start())
	println("advertising as", deviceName)

	show(m, cur)

	for {
		select {
		case c := <-colorCh:
			cur = c
			println("color:", c[0], c[1], c[2])
		case b := <-brightnessCh:
			m.SetBrightness(b)
			// 切り詰め後の値を読み出し用に反映する。
			// brightnessChar.Write() は WriteEvent を呼び出してしまい
			// ループになるため、値のスライスを直接書き換える。
			brightnessValue[0] = m.Brightness()
			println("brightness:", m.Brightness())
		}
		show(m, cur)
	}
}

func show(m *matrix.Matrix, c [3]byte) {
	m.Fill(color.RGBA{R: c[0], G: c[1], B: c[2], A: 255})
	m.Display()
}

// sendLatest は古い値が残っていれば捨てて、最新の値だけを送ります。
// スライダーを動かしたときに大量の書き込みが来ても詰まらないようにするためです。
func sendLatest[T any](ch chan T, v T) {
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- v:
	default:
	}
}

func mustParseUUID(s string) bluetooth.UUID {
	u, err := bluetooth.ParseUUID(s)
	if err != nil {
		panic(err)
	}
	return u
}

func must(action string, err error) {
	if err != nil {
		halt("failed to " + action + ": " + err.Error())
	}
}

func halt(msg string) {
	for {
		println(msg)
		time.Sleep(time.Second)
	}
}
