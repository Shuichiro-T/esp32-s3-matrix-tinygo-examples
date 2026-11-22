//go:build esp32s3

// Package matrix は ESP32-S3-Matrix の 8x8 RGB LED を扱うためのフレームバッファです。
//
// tinygo.org/x/drivers.Displayer を実装しているので、tinyfont や tinydraw から
// そのまま描画できます。
package matrix

import (
	"image/color"

	"esp32-s3-matrix-tinygo-examples/board"
	"esp32-s3-matrix-tinygo-examples/ws2812s3"
)

// ColorOrder は LED に送るバイトの並び順です。
type ColorOrder uint8

const (
	// RGB: Waveshare 公式デモ (Color / Game) は NEO_RGB を使っています。
	RGB ColorOrder = iota
	// GRB: 一般的な WS2812B の並び順です。色が入れ替わって見える場合はこちらを試してください。
	GRB
)

// Rotation は表示の回転 (時計回り) です。
type Rotation uint8

const (
	Rotate0 Rotation = iota
	Rotate90
	Rotate180
	Rotate270
)

// Matrix は 8x8 のフレームバッファと LED ドライバです。
type Matrix struct {
	dev        *ws2812s3.Device
	pixels     [board.NumLEDs]color.RGBA
	raw        [board.NumLEDs * 3]byte
	brightness uint8
	Order      ColorOrder
	Rotation   Rotation
}

// New は board.LEDPin に接続された LED マトリクスを初期化します。
// 初期輝度は 16/255 です。
func New() (*Matrix, error) {
	dev, err := ws2812s3.New(board.LEDPin)
	if err != nil {
		return nil, err
	}
	m := &Matrix{dev: dev, brightness: 16, Order: RGB}
	m.Clear()
	m.Display()
	return m, nil
}

// SetBrightness は全体の輝度 (0-255) を設定します。
// 基板の発熱を防ぐため board.MaxBrightness を上限として切り詰めます。
func (m *Matrix) SetBrightness(b uint8) {
	if b > board.MaxBrightness {
		b = board.MaxBrightness
	}
	m.brightness = b
}

// Brightness は現在の輝度を返します。
func (m *Matrix) Brightness() uint8 { return m.brightness }

// Size は drivers.Displayer の実装です。
func (m *Matrix) Size() (x, y int16) { return board.Width, board.Height }

// SetPixel は (x, y) の色を設定します。範囲外は無視します。
// 原点 (0, 0) は左上です。
func (m *Matrix) SetPixel(x, y int16, c color.RGBA) {
	if x < 0 || y < 0 || x >= board.Width || y >= board.Height {
		return
	}
	switch m.Rotation {
	case Rotate90:
		x, y = y, board.Width-1-x
	case Rotate180:
		x, y = board.Width-1-x, board.Height-1-y
	case Rotate270:
		x, y = board.Height-1-y, x
	}
	m.pixels[int(y)*board.Width+int(x)] = c
}

// Fill は全ピクセルを c で塗りつぶします。
func (m *Matrix) Fill(c color.RGBA) {
	for i := range m.pixels {
		m.pixels[i] = c
	}
}

// Clear は全ピクセルを消灯します (Display を呼ぶまで反映されません)。
func (m *Matrix) Clear() { m.Fill(color.RGBA{}) }

// Display はフレームバッファの内容を LED に送信します。
func (m *Matrix) Display() error {
	br := uint16(m.brightness)
	for i, c := range m.pixels {
		r := uint8(uint16(c.R) * br / 255)
		g := uint8(uint16(c.G) * br / 255)
		b := uint8(uint16(c.B) * br / 255)
		o := i * 3
		if m.Order == GRB {
			m.raw[o], m.raw[o+1], m.raw[o+2] = g, r, b
		} else {
			m.raw[o], m.raw[o+1], m.raw[o+2] = r, g, b
		}
	}
	_, err := m.dev.Write(m.raw[:])
	return err
}

// Wheel は 0-255 の値を虹色 (色相環) に変換します。
func Wheel(pos uint8) color.RGBA {
	switch {
	case pos < 85:
		return color.RGBA{R: 255 - pos*3, G: pos * 3, A: 255}
	case pos < 170:
		pos -= 85
		return color.RGBA{G: 255 - pos*3, B: pos * 3, A: 255}
	default:
		pos -= 170
		return color.RGBA{R: pos * 3, B: 255 - pos*3, A: 255}
	}
}
