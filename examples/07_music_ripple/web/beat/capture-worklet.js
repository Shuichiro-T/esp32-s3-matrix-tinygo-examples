// タブの音声をモノラル 22050Hz にして、ある程度まとめてからメインスレッドへ送る AudioWorklet。
//
// AudioContext を 22050Hz で作れた場合はそのまま送る。作れなかった場合（ブラウザが
// 違うサンプリング周波数の MediaStream をつなげない場合）は、ここで線形補間で間引く。
// その場合の折り返しノイズは、手前に置いたローパスフィルタで抑える。

const TARGET_RATE = 22050;
const BATCH = 1024; // 送る単位（約 46ms）

class CaptureProcessor extends AudioWorkletProcessor {
  constructor() {
    super();
    this.ratio = sampleRate / TARGET_RATE; // 入力何サンプルで出力 1 サンプルか
    this.pos = 0;                          // 次の出力サンプルの入力上の位置（端数あり）
    this.prev = 0;                         // 前のブロックの最後のサンプル（補間用）
    this.out = new Float32Array(BATCH);
    this.outLen = 0;
    this.startTime = -1;                   // 最初のサンプルの AudioContext 上の時刻
    this.sent = 0;                         // これまでに送ったサンプル数
  }

  process(inputs) {
    const ch = inputs[0];
    if (!ch || ch.length === 0) return true;
    const n = ch[0].length;
    if (this.startTime < 0) this.startTime = currentFrame / sampleRate;

    // ステレオをモノラルに
    const mono = new Float32Array(n);
    for (let c = 0; c < ch.length; c++) {
      const d = ch[c];
      for (let i = 0; i < n; i++) mono[i] += d[i];
    }
    if (ch.length > 1) for (let i = 0; i < n; i++) mono[i] /= ch.length;

    if (this.ratio === 1) {
      for (let i = 0; i < n; i++) this.emit(mono[i]);
    } else {
      // pos は「このブロックの先頭を 0 とした入力位置」。-1 は前のブロックの最後のサンプル。
      while (this.pos < n - 1) {
        const i = Math.floor(this.pos), f = this.pos - i;
        const a = i < 0 ? this.prev : mono[i];
        this.emit(a + (mono[i + 1] - a) * f);
        this.pos += this.ratio;
      }
      this.pos -= n;
      this.prev = mono[n - 1];
    }
    return true;
  }

  emit(v) {
    this.out[this.outLen++] = v;
    if (this.outLen === BATCH) {
      // time: このまとまりの先頭のサンプルの AudioContext 上の時刻
      const time = this.startTime + this.sent / TARGET_RATE;
      this.port.postMessage({ samples: this.out, time }, [this.out.buffer]);
      this.sent += BATCH;
      this.out = new Float32Array(BATCH);
      this.outLen = 0;
    }
  }
}

registerProcessor("capture-processor", CaptureProcessor);
