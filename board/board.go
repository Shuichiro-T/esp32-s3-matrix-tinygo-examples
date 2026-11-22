// Package board は Waveshare ESP32-S3-Matrix のピン配置と定数をまとめたものです。
//
// ピン番号は Waveshare 公式デモ (ESP32-S3-Matrix-Demo.zip) に基づきます。
package board

import "machine"

const (
	// LEDPin は 8x8 RGB LED マトリクス (WS2812 互換) のデータピンです。
	LEDPin = machine.GPIO14

	// IMU (QMI8658) が接続されている I2C ピンです。
	SDAPin = machine.GPIO11
	SCLPin = machine.GPIO12

	// QMI8658Address は IMU の I2C アドレスです (SA0=High)。
	QMI8658Address = 0x6B
)

const (
	Width   = 8
	Height  = 8
	NumLEDs = Width * Height
)

// MaxBrightness は本リポジトリのサンプルで許可する輝度 (0-255) の上限です。
//
// Waveshare は「輝度を上げすぎると基板温度が急上昇し、破損するおそれがある」
// と注意喚起しています。公式デモの setBrightness も 40〜60 程度です。
const MaxBrightness = 64
