// 07_music_ripple: ブラウザで再生している YouTube の音に合わせて LED を光らせます。
//
//   - 低音のビート: 中心から四角い波紋が 2x2 → 4x4 → 6x6 → 8x8 と外へ広がる
//   - 音の出だし（オンセット）: ランダムな 1 点が星のように光って消える
//   - 色は時間とともに少しずつ虹色に変わる
//
// 音の解析（FFT とビート検出）はブラウザ側の web/index.html で行い、
// 結果だけを Bluetooth LE で受け取ります。ボード側はアニメーションを描きます。
// 使い方は README の「07_music_ripple」の項を参照してください。
package main

import (
	"image/color"
	"time"

	"tinygo.org/x/bluetooth"

	"esp32-s3-matrix-tinygo-examples/board"
	"esp32-s3-matrix-tinygo-examples/matrix"
)

// GATT の UUID。web/index.html と同じ値にしてください。
var (
	serviceUUID = mustParseUUID("6e400011-7a3c-4f1b-9d2e-5c8b1f0a3e01")
	// 音の情報: 4 バイト (低音レベル 0〜255, 低音ビート番号, オンセット番号, 強さ 0〜255)。書き込みのみ。
	audioUUID = mustParseUUID("6e400012-7a3c-4f1b-9d2e-5c8b1f0a3e01")
	// 輝度: 1 バイト (0〜255)。上限は board.MaxBrightness で切り詰められる。
	brightnessUUID = mustParseUUID("6e400013-7a3c-4f1b-9d2e-5c8b1f0a3e01")
)

const deviceName = "S3-Matrix"

// アニメーションの調整値
const (
	frameInterval = 25 * time.Millisecond // 約 40fps
	numRings      = 4                     // 四角い輪の数（中心の 2x2 が 0、外周が 3）

	maxRipples  = 4    // 同時に表示する波紋の数
	rippleSpeed = 0.18 // 1 フレームで波紋が進む輪の数（約 0.55 秒で外周に届く）
	rippleTail  = 1.6  // 波紋の後ろに残る残像の長さ（輪の数）
	rippleDecay = 0.97 // 1 フレームごとの波紋の減衰率

	maxStars  = 8    // 同時に表示する星の数
	starDecay = 0.88 // 1 フレームごとの星の減衰率
	starHue   = 128  // 星の色相を背景・波紋からずらす量（256 で 1 周。128 は補色）

	hueSpeed    = 0.35 // 1 フレームで色相が進む量（256 で 1 周、約 18 秒）
	hueSpread   = 18   // 輪 1 つあたりの色相のずれ
	glowLevel   = 0.15 // 低音レベルに応じた背景の明るさ（最大値）
	idleTimeout = 1500 * time.Millisecond
)

// audioPacket はブラウザから届く音の情報です。
type audioPacket struct {
	bass     uint8 // 低音の大きさ (0〜255)
	bassBeat uint8 // 低音ビートを検出するたびに 1 増える番号
	onset    uint8 // オンセットを検出するたびに 1 増える番号
	strength uint8 // 直近のビート / オンセットの強さ (0〜255)
}

type ripple struct {
	active   bool
	pos      float32 // 波紋の先端がいる輪（0〜numRings）
	strength float32 // 0〜1
	hue      float32 // 0〜256
}

type star struct {
	idx      int     // ピクセルの番号 (y*8 + x)
	strength float32 // 0〜1。0 で消えている
	hue      float32
}

var adapter = bluetooth.DefaultAdapter

// BLE のコールバックからメインループへ値を渡すチャネル。
// コールバック内では LED の送信（割り込み禁止を伴う）を行わない。
var (
	audioCh      = make(chan audioPacket, 1)
	brightnessCh = make(chan uint8, 1)
)

// ring は各ピクセルが中心から数えて何番目の四角い輪にあるか (0〜3) です。
var ring [board.NumLEDs]uint8

func main() {
	time.Sleep(time.Second) // シリアルモニタの接続待ち

	// 中心 (3.5, 3.5) からのチェビシェフ距離（縦横の差の大きい方）で輪を決める。
	for y := 0; y < board.Height; y++ {
		for x := 0; x < board.Width; x++ {
			ring[y*board.Width+x] = uint8(max(absDiff(x), absDiff(y)))
		}
	}

	m, err := matrix.New()
	if err != nil {
		halt("matrix init failed: " + err.Error())
	}
	m.SetBrightness(24)
	brightnessValue := []byte{m.Brightness()} // GATT の読み出し値としてそのまま使われる

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

	var audioChar, brightnessChar bluetooth.Characteristic
	must("add service", adapter.AddService(&bluetooth.Service{
		UUID: serviceUUID,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &audioChar,
				UUID:   audioUUID,
				// Write Without Response は ESP32 の HCI 実装では無視されるため、
				// 応答ありの Write だけを受け付ける。
				Flags: bluetooth.CharacteristicWritePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					if offset != 0 || len(value) != 4 {
						return
					}
					sendLatest(audioCh, audioPacket{
						bass: value[0], bassBeat: value[1], onset: value[2], strength: value[3],
					})
				},
			},
			{
				Handle: &brightnessChar,
				UUID:   brightnessUUID,
				Value:  brightnessValue,
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

	// tinygo.org/x/bluetooth v0.17.0 の HCI 実装（ESP32 で使われるもの）は、
	// アドバタイズに 128 ビットのサービス UUID を載せられない（壊れた項目が入るだけ）。
	// そのため名前だけをアドバタイズし、ブラウザ側は名前で探す。
	adv := adapter.DefaultAdvertisement()
	must("configure advertisement", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName: deviceName,
	}))
	must("start advertisement", adv.Start())
	println("advertising as", deviceName)

	var (
		ripples   [maxRipples]ripple
		stars     [maxStars]star
		hue       float32
		glow      float32 // 背景の明るさ（低音レベルを滑らかにしたもの）
		bassLevel float32 // 最後に届いた低音レベル (0〜1)
		last      audioPacket
		lastRecv  time.Time // 最後に音の情報を受け取った時刻
		lastIdle  time.Time // 待機中の波紋を最後に出した時刻
		rng       = uint32(time.Now().UnixNano()) | 1
	)

	addRipple := func(strength float32) {
		// 空きがなければ、いちばん弱い波紋を置き換える。
		idx := 0
		for i := range ripples {
			if !ripples[i].active {
				idx = i
				break
			}
			if ripples[i].strength < ripples[idx].strength {
				idx = i
			}
		}
		ripples[idx] = ripple{active: true, strength: strength, hue: hue}
	}

	addStar := func(strength float32) {
		// いちばん暗い星を置き換える。
		idx := 0
		for i := range stars {
			if stars[i].strength < stars[idx].strength {
				idx = i
			}
		}
		rng = xorshift(rng)
		stars[idx] = star{idx: int(rng % board.NumLEDs), strength: strength, hue: hue + starHue}
	}

	ticker := time.NewTicker(frameInterval)
	for {
		select {
		case p := <-audioCh:
			now := time.Now()
			s := 0.35 + 0.65*float32(p.strength)/255
			// 待機状態から戻った直後は、番号の差だけで光らせない。
			if now.Sub(lastRecv) < idleTimeout {
				if p.bassBeat != last.bassBeat {
					addRipple(s)
				}
				if p.onset != last.onset {
					addStar(s)
				}
			}
			last = p
			bassLevel = float32(p.bass) / 255
			lastRecv = now
			continue
		case b := <-brightnessCh:
			m.SetBrightness(b)
			// 切り詰め後の値を読み出し用に反映する。
			// brightnessChar.Write() は WriteEvent を呼び出してしまい
			// ループになるため、値のスライスを直接書き換える。
			brightnessValue[0] = m.Brightness()
			println("brightness:", m.Brightness())
			continue
		case <-ticker.C:
		}

		// 音の情報が途絶えたら待機表示: 弱い波紋をゆっくり出す。
		now := time.Now()
		if now.Sub(lastRecv) > idleTimeout {
			bassLevel = 0
			if now.Sub(lastIdle) > 2*time.Second {
				addRipple(0.3)
				lastIdle = now
			}
		}

		glow += (bassLevel - glow) * 0.3
		hue += hueSpeed
		if hue >= 256 {
			hue -= 256
		}
		for i := range ripples {
			r := &ripples[i]
			if !r.active {
				continue
			}
			r.pos += rippleSpeed
			r.strength *= rippleDecay
			// 残像が外周を抜けきったら消す。
			if r.pos > numRings+rippleTail || r.strength < 0.02 {
				r.active = false
			}
		}
		for i := range stars {
			stars[i].strength *= starDecay
		}

		render(m, &ripples, &stars, hue, glow)
		m.Display()
	}
}

// render は背景・波紋・星をフレームバッファに描きます。
func render(m *matrix.Matrix, ripples *[maxRipples]ripple, stars *[maxStars]star, hue, glow float32) {
	var buf [board.NumLEDs][3]float32

	for i := range buf {
		k := float32(ring[i])
		h := k * hueSpread

		// 背景: 低音の大きさに合わせて、中心ほど明るく光る。
		if bg := glow * glowLevel * (1 - k/numRings); bg > 0 {
			addColor(&buf[i], hue+h, bg)
		}

		// 波紋: 先端（pos の位置の輪）がいちばん明るく、内側に残像が残る。
		// 先端より外側はまだ光らせないので、輪の境目がくっきりする。
		for j := range ripples {
			rp := &ripples[j]
			if !rp.active {
				continue
			}
			// 輪 k は、先端が k に届いた時点で光り始める。
			behind := rp.pos - k
			if behind < 0 || behind >= rippleTail {
				continue
			}
			addColor(&buf[i], rp.hue+h, rp.strength*(1-behind/rippleTail))
		}
	}

	// 星
	for j := range stars {
		st := &stars[j]
		if st.strength < 0.02 {
			continue
		}
		addColor(&buf[st.idx], st.hue, st.strength)
	}

	for i, c := range buf {
		m.SetPixel(int16(i%board.Width), int16(i/board.Width),
			color.RGBA{R: clamp8(c[0]), G: clamp8(c[1]), B: clamp8(c[2]), A: 255})
	}
}

// addColor は色相 h の虹色を、明るさ v (0〜1) で足し込みます。
func addColor(dst *[3]float32, h, v float32) {
	c := matrix.Wheel(uint8(int(h) & 0xff))
	dst[0] += float32(c.R) * v
	dst[1] += float32(c.G) * v
	dst[2] += float32(c.B) * v
}

// absDiff は座標 0〜7 の、中心 3.5 からの距離を 0〜3 の整数で返します。
func absDiff(v int) int {
	if v < 4 {
		return 3 - v
	}
	return v - 4
}

func xorshift(x uint32) uint32 {
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	return x
}

func clamp8(v float32) uint8 {
	if v >= 255 {
		return 255
	}
	if v <= 0 {
		return 0
	}
	return uint8(v)
}

// sendLatest は古い値が残っていれば捨てて、最新の値だけを送ります。
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
