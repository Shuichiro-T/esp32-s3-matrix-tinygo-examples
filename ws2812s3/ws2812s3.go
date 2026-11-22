//go:build esp32s3

// Package ws2812s3 は ESP32-S3 用の最小限の WS2812 (NeoPixel) ドライバです。
//
// TinyGo 0.41 時点の tinygo.org/x/drivers/ws2812 は ESP32-S3 に対応していない
// (machine.CPUFrequency が未定義でビルドできない) ため、CPU のサイクルカウンタ
// (CCOUNT レジスタ) で時間を計りながら GPIO をビットバンギングします。
//
// 送信中は割り込みを禁止します。8x8 (64 LED) の送信時間は約 2ms です。
package ws2812s3

import (
	"device"
	"errors"
	"machine"
	"runtime/interrupt"
	"runtime/volatile"
)

// WS2812 のタイミング (ナノ秒)。データシートの許容範囲の中央付近の値です。
const (
	t0hNs    = 350
	t1hNs    = 750
	periodNs = 1250
)

// Device は WS2812 が接続された 1 本のデータピンを表します。
type Device struct {
	set, clr *uint32
	mask     uint32

	// CPU サイクル数に換算したタイミング
	t0h, t1h, period uint32
}

// New は pin を出力に設定し、Device を返します。
func New(pin machine.Pin) (*Device, error) {
	freq, err := machine.GetCPUFrequency()
	if err != nil {
		return nil, err
	}
	if freq < 80e6 {
		return nil, errors.New("ws2812s3: CPU clock too slow")
	}
	pin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pin.Low()

	set, mask := pin.PortMaskSet()
	clr, _ := pin.PortMaskClear()
	mhz := freq / 1e6
	return &Device{
		set:    set,
		clr:    clr,
		mask:   mask,
		t0h:    t0hNs * mhz / 1000,
		t1h:    t1hNs * mhz / 1000,
		period: periodNs * mhz / 1000,
	}, nil
}

// Write は buf の各バイトを MSB から順に WS2812 プロトコルで送信します。
// バイトの並び (GRB / RGB など) は呼び出し側で整えてください。
func (d *Device) Write(buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}
	state := interrupt.Disable()
	// コードはフラッシュ上にありキャッシュ経由で実行されるため、最初の 1 回は
	// キャッシュミスでタイミングが崩れることがあります。そこで mask=0 (ピンが
	// 変化しない) で一度空回しして命令キャッシュを温めてから本送信します。
	n := len(buf)
	if n > 2 {
		n = 2
	}
	d.send(buf[:n], 0)
	d.send(buf, d.mask)
	interrupt.Restore(state)
	return len(buf), nil
}

func (d *Device) send(buf []byte, mask uint32) {
	set, clr := d.set, d.clr
	t0h, t1h, period := d.t0h, d.t1h, d.period
	start := ccount()
	for _, b := range buf {
		for i := 0; i < 8; i++ {
			t := t0h
			if b&0x80 != 0 {
				t = t1h
			}
			b <<= 1
			for ccount()-start < period {
			}
			start = ccount()
			volatile.StoreUint32(set, mask)
			for ccount()-start < t {
			}
			volatile.StoreUint32(clr, mask)
		}
	}
	// 次のフレームのためのリセット期間 (>= 50us は呼び出し側の間隔で十分確保される)
}

// ccount は Xtensa のサイクルカウンタを読み出します。
func ccount() uint32 {
	return uint32(device.AsmFull("rsr.ccount {}", nil))
}
