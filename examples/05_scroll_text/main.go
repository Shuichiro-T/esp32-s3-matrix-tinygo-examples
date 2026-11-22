// 05_scroll_text: tinyfont を使って文字列を右から左へスクロール表示します。
//
// matrix.Matrix は drivers.Displayer を実装しているので、tinyfont / tinydraw
// の描画関数をそのまま使えます。
package main

import (
	"image/color"
	"time"

	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/proggy"

	"esp32-s3-matrix-tinygo-examples/matrix"
)

const message = "Hello, TinyGo!"

func main() {
	m, err := matrix.New()
	if err != nil {
		for {
			println("matrix init failed:", err.Error())
			time.Sleep(time.Second)
		}
	}
	m.SetBrightness(20)

	font := &proggy.TinySZ8pt7b
	_, w := tinyfont.LineWidth(font, message)
	width := int16(w)

	for {
		for x := int16(8); x > -width; x-- {
			m.Clear()
			// y はベースライン位置。8px 高のマトリクスに収まるよう 6 を指定
			tinyfont.WriteLineColors(m, font, x, 6, message, []color.RGBA{
				{R: 255, A: 255}, {R: 255, G: 160, A: 255}, {G: 255, A: 255},
				{G: 200, B: 255, A: 255}, {B: 255, A: 255}, {R: 200, B: 255, A: 255},
			})
			m.Display()
			time.Sleep(70 * time.Millisecond)
		}
	}
}
