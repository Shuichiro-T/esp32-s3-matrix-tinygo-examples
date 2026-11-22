// 03_imu_serial: QMI8658 の加速度・角速度・温度を USB シリアルに出力します。
//
//	tinygo monitor
//
// で確認できます。
package main

import (
	"machine"
	"time"

	"esp32-s3-matrix-tinygo-examples/board"
	"esp32-s3-matrix-tinygo-examples/qmi8658"
)

func main() {
	time.Sleep(2 * time.Second) // USB シリアル接続を待つ

	i2c := machine.I2C0
	if err := i2c.Configure(machine.I2CConfig{
		SDA:       board.SDAPin,
		SCL:       board.SCLPin,
		Frequency: 400 * machine.KHz,
	}); err != nil {
		println("i2c configure failed:", err.Error())
	}

	imu := qmi8658.New(i2c, board.QMI8658Address)
	for {
		if err := imu.Configure(); err != nil {
			println(err.Error())
			time.Sleep(time.Second)
			continue
		}
		break
	}
	println("QMI8658 ready")

	for {
		ax, ay, az, err1 := imu.ReadAcceleration()
		gx, gy, gz, err2 := imu.ReadRotation()
		t, err3 := imu.ReadTemperature()
		if err1 != nil || err2 != nil || err3 != nil {
			println("read error")
		} else {
			// 加速度: mg, 角速度: mdps, 温度: m℃
			println("acc[mg]", ax, ay, az, " gyro[mdps]", gx, gy, gz, " temp[mC]", t)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
