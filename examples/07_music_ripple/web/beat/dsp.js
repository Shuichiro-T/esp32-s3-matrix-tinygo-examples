// Beat This! の入力（スペクトログラム）を作るための信号処理。
//
// beat_this の前処理（torchaudio の MelSpectrogram）と同じく、22050Hz の音を
//   窓長 1024 点（周期ハン窓）、ホップ 441 点（20ms = 50 フレーム/秒）、各フレームは中心合わせ
// で切り出して rfft の振幅を求める。正規化・メルフィルタ・log1p は ONNX モデル側で行う。

export const SAMPLE_RATE = 22050;
export const N_FFT = 1024;
export const HOP = 441;
export const N_BINS = N_FFT / 2 + 1;
export const FPS = SAMPLE_RATE / HOP; // 50

// 基数 2 の FFT（サイズ固定）。re / im をその場で書き換える。
class FFT {
  constructor(n) {
    this.n = n;
    this.rev = new Uint32Array(n);
    const bits = Math.log2(n);
    for (let i = 0; i < n; i++) {
      let r = 0;
      for (let b = 0; b < bits; b++) r |= ((i >> b) & 1) << (bits - 1 - b);
      this.rev[i] = r;
    }
    this.cos = new Float64Array(n / 2);
    this.sin = new Float64Array(n / 2);
    for (let i = 0; i < n / 2; i++) {
      this.cos[i] = Math.cos((2 * Math.PI * i) / n);
      this.sin[i] = -Math.sin((2 * Math.PI * i) / n);
    }
  }

  transform(re, im) {
    const n = this.n, rev = this.rev;
    for (let i = 0; i < n; i++) {
      const j = rev[i];
      if (j > i) {
        let t = re[i]; re[i] = re[j]; re[j] = t;
        t = im[i]; im[i] = im[j]; im[j] = t;
      }
    }
    for (let size = 2; size <= n; size <<= 1) {
      const half = size >> 1, step = n / size;
      for (let start = 0; start < n; start += size) {
        for (let k = 0; k < half; k++) {
          const wr = this.cos[k * step], wi = this.sin[k * step];
          const a = start + k, b = a + half;
          const xr = re[b] * wr - im[b] * wi;
          const xi = re[b] * wi + im[b] * wr;
          re[b] = re[a] - xr; im[b] = im[a] - xi;
          re[a] += xr; im[a] += xi;
        }
      }
    }
  }
}

// 音声サンプルを少しずつ受け取り、フレーム（513 本の振幅）を順に作る。
// フレーム j は、ストリーム先頭からのサンプル位置 j*HOP を中心とする。
// 先頭（位置が負になる部分）は 0 として扱う。
export class SpectrogramStream {
  constructor(maxFrames) {
    this.maxFrames = maxFrames;            // 保持するフレーム数（リングバッファ）
    this.frames = new Float32Array(maxFrames * N_BINS);
    this.frameCount = 0;                    // これまでに作ったフレームの総数
    this.buf = new Float32Array(N_FFT * 4); // 未処理のサンプル
    this.bufStart = -N_FFT / 2;             // buf[0] のストリーム上の位置
    this.bufLen = N_FFT / 2;                // 先頭の半窓ぶんは 0 で埋めておく
    this.fft = new FFT(N_FFT);
    this.re = new Float64Array(N_FFT);
    this.im = new Float64Array(N_FFT);
    this.win = new Float64Array(N_FFT);
    for (let i = 0; i < N_FFT; i++) this.win[i] = 0.5 - 0.5 * Math.cos((2 * Math.PI * i) / N_FFT);
  }

  // samples: 22050Hz のモノラル音声。作れたフレームの数を返す。
  push(samples) {
    let made = 0;
    let off = 0;
    while (off < samples.length) {
      const n = Math.min(samples.length - off, this.buf.length - this.bufLen);
      this.buf.set(samples.subarray(off, off + n), this.bufLen);
      this.bufLen += n;
      off += n;
      // 次のフレームの窓 [j*HOP - N_FFT/2, j*HOP + N_FFT/2) が揃うたびに処理する。
      for (;;) {
        const begin = this.frameCount * HOP - N_FFT / 2 - this.bufStart;
        if (begin + N_FFT > this.bufLen) break;
        this.computeFrame(begin);
        made++;
      }
      // 使い終わったサンプルを捨てる。
      const keep = this.frameCount * HOP - N_FFT / 2 - this.bufStart;
      if (keep > 0) {
        this.buf.copyWithin(0, keep, this.bufLen);
        this.bufLen -= keep;
        this.bufStart += keep;
      }
    }
    return made;
  }

  computeFrame(begin) {
    const re = this.re, im = this.im, win = this.win, buf = this.buf;
    for (let i = 0; i < N_FFT; i++) {
      re[i] = buf[begin + i] * win[i];
      im[i] = 0;
    }
    this.fft.transform(re, im);
    const base = (this.frameCount % this.maxFrames) * N_BINS;
    for (let k = 0; k < N_BINS; k++) this.frames[base + k] = Math.hypot(re[k], im[k]);
    this.frameCount++;
  }

  // 最新の n フレーム（時間順）を、新しい (n * N_BINS) の配列にコピーして返す。
  // フレームがまだ n 個ない場合は、ある分だけを返す。戻り値の start は先頭フレームの番号。
  latest(n) {
    n = Math.min(n, this.frameCount, this.maxFrames);
    const start = this.frameCount - n;
    const data = new Float32Array(n * N_BINS);
    for (let i = 0; i < n; i++) {
      const src = ((start + i) % this.maxFrames) * N_BINS;
      data.set(this.frames.subarray(src, src + N_BINS), i * N_BINS);
    }
    return { start, n, data };
  }
}
