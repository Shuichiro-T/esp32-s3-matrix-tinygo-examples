// 04_tilt_dot: 基板を傾けると光の点が転がるサンプルです
// (公式デモの "Game" に相当)。点の通った跡は徐々に消えていきます。
//
// 傾けた方向と点の動きが逆の場合は invertX / invertY を変更してください。
package main

import (
	"image/color"
	"machine"
	"time"

	"esp32-s3-matrix-tinygo-examples/board"
	"esp32-s3-matrix-tinygo-examples/matrix"
	"esp32-s3-matrix-tinygo-examples/qmi8658"
)

const (
	invertX = true
	invertY = false
	swapXY  = true // IMU の X/Y 軸は LED の並びに対して 90° 回転している
)

func main() {
	m, err := matrix.New()
	if err != nil {
		for {
			println("matrix init failed:", err.Error())
			time.Sleep(time.Second)
		}
	}
	m.SetBrightness(24)

	i2c := machine.I2C0
	i2c.Configure(machine.I2CConfig{SDA: board.SDAPin, SCL: board.SCLPin, Frequency: 400 * machine.KHz})
	imu := qmi8658.New(i2c, board.QMI8658Address)
	for imu.Configure() != nil {
		// IMU が見つからない間は赤く点滅
		m.Fill(color.RGBA{R: 255, A: 255})
		m.Display()
		time.Sleep(200 * time.Millisecond)
		m.Clear()
		m.Display()
		time.Sleep(200 * time.Millisecond)
	}

	// 位置・速度は 1/1000 ピクセル単位の固定小数点で扱う
	const scale = 1000
	const maxPos = (board.Width - 1) * scale
	px, py := int32(maxPos/2), int32(maxPos/2)
	var vx, vy int32

	// 残像用のバッファ
	var trail [board.Height][board.Width]uint8

	for {
		ax, ay, _, err := imu.ReadAcceleration()
		if err == nil {
			if swapXY {
				ax, ay = ay, ax
			}
			if invertX {
				ax = -ax
			}
			if invertY {
				ay = -ay
			}
			// 加速度 (mg) を速度に加算し、摩擦で減衰させる
			vx = (vx + ax/8) * 9 / 10
			vy = (vy + ay/8) * 9 / 10
			px, vx = bounce(px+vx, vx, maxPos)
			py, vy = bounce(py+vy, vy, maxPos)
		}

		for y := range trail {
			for x := range trail[y] {
				trail[y][x] = trail[y][x] * 3 / 4
			}
		}
		cx, cy := (px+scale/2)/scale, (py+scale/2)/scale
		trail[cy][cx] = 255

		for y := int16(0); y < board.Height; y++ {
			for x := int16(0); x < board.Width; x++ {
				v := trail[y][x]
				m.SetPixel(x, y, color.RGBA{R: v / 4, G: v / 2, B: v, A: 255})
			}
		}
		m.SetPixel(int16(cx), int16(cy), color.RGBA{R: 255, G: 255, B: 255, A: 255})
		m.Display()
		time.Sleep(20 * time.Millisecond)
	}
}

// bounce は壁に当たったら位置を範囲内に戻し、速度を反転・減衰させます。
func bounce(p, v, max int32) (int32, int32) {
	if p < 0 {
		return 0, -v / 2
	}
	if p > max {
		return max, -v / 2
	}
	return p, v
}
