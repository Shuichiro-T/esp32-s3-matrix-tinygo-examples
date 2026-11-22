// 01_color_check: 赤 → 緑 → 青 → 白 の順に全面を点灯し、
// LED の色順 (RGB / GRB) と配線が正しいか確認するサンプルです。
//
// 表示される色が「赤 → 緑 → 青」でなく「緑 → 赤 → 青」になる場合は
// m.Order = matrix.GRB に変更してください。
package main

import (
	"image/color"
	"time"

	"esp32-s3-matrix-tinygo-examples/matrix"
)

func main() {
	time.Sleep(time.Second) // USB シリアル接続を待つ

	m, err := matrix.New()
	if err != nil {
		for {
			println("matrix init failed:", err.Error())
			time.Sleep(time.Second)
		}
	}
	m.SetBrightness(16)
	// m.Order = matrix.GRB

	steps := []struct {
		name string
		c    color.RGBA
	}{
		{"red", color.RGBA{R: 255, A: 255}},
		{"green", color.RGBA{G: 255, A: 255}},
		{"blue", color.RGBA{B: 255, A: 255}},
		{"white", color.RGBA{R: 255, G: 255, B: 255, A: 255}},
	}

	for {
		for _, s := range steps {
			println(s.name)
			m.Fill(s.c)
			m.Display()
			time.Sleep(time.Second)
		}
		// 左上 (0,0) から 1 ピクセルずつ点灯して座標の向きを確認
		println("scan")
		m.Clear()
		for y := int16(0); y < 8; y++ {
			for x := int16(0); x < 8; x++ {
				m.SetPixel(x, y, color.RGBA{R: 255, G: 128, A: 255})
				m.Display()
				time.Sleep(30 * time.Millisecond)
			}
		}
	}
}
