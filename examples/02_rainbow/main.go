// 02_rainbow: 斜め方向に流れる虹色アニメーションです。
package main

import (
	"time"

	"esp32-s3-matrix-tinygo-examples/matrix"
)

func main() {
	m, err := matrix.New()
	if err != nil {
		for {
			println("matrix init failed:", err.Error())
			time.Sleep(time.Second)
		}
	}
	m.SetBrightness(20)

	var offset uint8
	for {
		for y := int16(0); y < 8; y++ {
			for x := int16(0); x < 8; x++ {
				m.SetPixel(x, y, matrix.Wheel(uint8((x+y)*16)+offset))
			}
		}
		m.Display()
		offset += 3
		time.Sleep(30 * time.Millisecond)
	}
}
