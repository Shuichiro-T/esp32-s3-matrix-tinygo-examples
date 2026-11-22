// Package qmi8658 は ESP32-S3-Matrix に搭載された 6 軸 IMU
// QMI8658 (QST) の最小限のドライバです。
//
// tinygo.org/x/drivers/qmi8658c も存在しますが、v0.36.0 時点では
// 加速度/角速度の下位バイトの読み出し位置に不具合があるため、
// ここでは必要な機能だけを自前で実装しています。
package qmi8658

import (
	"errors"
	"time"

	"tinygo.org/x/drivers"
)

// レジスタ
const (
	regWhoAmI = 0x00
	regCtrl1  = 0x02
	regCtrl2  = 0x03 // 加速度設定
	regCtrl3  = 0x04 // ジャイロ設定
	regCtrl5  = 0x06 // ローパスフィルタ
	regCtrl7  = 0x08 // センサ有効化
	regTempL  = 0x33
	regAxL    = 0x35

	whoAmIValue = 0x05
)

// 設定値
const (
	ctrl1AddrAutoInc = 0x40 // シリアルインタフェースのアドレス自動インクリメント
	ctrl2Acc4G       = 0x10 // ±4g
	ctrl3Gyro512DPS  = 0x50 // ±512 dps
	odr250Hz         = 0x05 // 出力データレート (6DOF 時はジャイロに従い ~224Hz)
	ctrl7AccGyroEn   = 0x03

	accLSBPerG    = 8192 // ±4g 時
	gyroLSBPerDPS = 64   // ±512dps 時
)

// Device は I2C 接続の QMI8658 です。
type Device struct {
	bus     drivers.I2C
	Address uint16
	buf     [12]byte
}

// New は Device を返します。address は通常 board.QMI8658Address (0x6B) です。
func New(bus drivers.I2C, address uint16) *Device {
	return &Device{bus: bus, Address: address}
}

// Connected は WHO_AM_I レジスタを読み、QMI8658 が応答するか確認します。
func (d *Device) Connected() bool {
	v, err := d.read8(regWhoAmI)
	return err == nil && v == whoAmIValue
}

// Configure は加速度 ±4g・ジャイロ ±512dps・約 224Hz で計測を開始します。
func (d *Device) Configure() error {
	if !d.Connected() {
		return errors.New("qmi8658: device not found")
	}
	for _, rv := range [][2]byte{
		{regCtrl1, ctrl1AddrAutoInc},
		{regCtrl2, ctrl2Acc4G | odr250Hz},
		{regCtrl3, ctrl3Gyro512DPS | odr250Hz},
		{regCtrl5, 0x00},
		{regCtrl7, ctrl7AccGyroEn},
	} {
		if err := d.tx(rv[:], nil); err != nil {
			return err
		}
	}
	return nil
}

// ReadAcceleration は加速度を mg (1g = 1000) で返します。
func (d *Device) ReadAcceleration() (x, y, z int32, err error) {
	if err = d.readRegs(regAxL, d.buf[:6]); err != nil {
		return
	}
	x = int32(le16(d.buf[0:])) * 1000 / accLSBPerG
	y = int32(le16(d.buf[2:])) * 1000 / accLSBPerG
	z = int32(le16(d.buf[4:])) * 1000 / accLSBPerG
	return
}

// ReadRotation は角速度を mdeg/s (1dps = 1000) で返します。
func (d *Device) ReadRotation() (x, y, z int32, err error) {
	if err = d.readRegs(regAxL+6, d.buf[6:12]); err != nil {
		return
	}
	x = int32(le16(d.buf[6:])) * 1000 / gyroLSBPerDPS
	y = int32(le16(d.buf[8:])) * 1000 / gyroLSBPerDPS
	z = int32(le16(d.buf[10:])) * 1000 / gyroLSBPerDPS
	return
}

// ReadTemperature は温度をミリ℃で返します。
func (d *Device) ReadTemperature() (int32, error) {
	var b [2]byte
	if err := d.readRegs(regTempL, b[:]); err != nil {
		return 0, err
	}
	return int32(le16(b[:])) * 1000 / 256, nil
}

func (d *Device) read8(reg byte) (byte, error) {
	var b [1]byte
	err := d.readRegs(reg, b[:])
	return b[0], err
}

func (d *Device) readRegs(reg byte, dst []byte) error {
	return d.tx([]byte{reg}, dst)
}

// txGap は I2C トランザクション間に空ける待ち時間です。
//
// TinyGo (v0.41 時点) の ESP32-S3 用 I2C 実装では、読み出し直後に次の
// トランザクションを開始すると、前回の完了割り込みを今回の完了と誤認して
// 空の RX FIFO を読んでしまい、以降のデータが 0xFF などにずれ続けます
// (ReadAcceleration の直後に ReadRotation を呼ぶと発生)。
// 数 µs の待ちで解消するため、余裕を持たせて 50µs 空けています。
const txGap = 50 * time.Microsecond

func (d *Device) tx(w, r []byte) error {
	err := d.bus.Tx(d.Address, w, r)
	time.Sleep(txGap)
	return err
}

func le16(b []byte) int16 { return int16(uint16(b[0]) | uint16(b[1])<<8) }
