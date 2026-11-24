// Beat This! の出力（フレームごとのロジット）から、これから来るビートを予測する。
//
// Beat This! は曲全体（最大 30 秒）を前後両方向から見て判断するオフライン用のモデルなので、
// 直近の数秒だけを入力にすると、右端（＝今）付近のビートは未来の音が見えないぶん不確かになる。
// そこで次のように使う。
//
//   1. 直近 W フレームで推論を繰り返し、新しい結果でロジットを上書きしていく。
//   2. 右端から guard フレーム以上前の部分（十分な「未来」が見えている部分）だけで
//      ビートを確定する。確定のしかたは論文の最小限の後処理と同じ
//      （±70ms の最大値で、確率 0.5 超え）。
//   3. 確定したビートの間隔の中央値をテンポ、最小二乗の当てはめを位相として、
//      その先のビートの時刻を予測する。ダウンビートも同じように確定し、小節の拍数を数えて予測する。
//
// 時刻はすべて「音声ストリームの先頭からの秒数」で扱う。

import { FPS } from "./dsp.js";

const DEFAULTS = {
  guardFrames: 25,     // 右端からこのフレーム数より前だけでビートを確定する（25 = 0.5 秒）
  historySec: 6,       // テンポと位相の推定に使う期間
  minBeats: 4,         // 予測を始めるのに必要な確定ビートの数
  maxGapBeats: 2.5,    // 最後の確定ビートからこの拍数以上たったら予測をやめる（曲の切れ目など）
  minPeriod: 60 / 300, // 扱うテンポの範囲（秒/拍）
  maxPeriod: 60 / 50,
};

export class BeatTracker {
  constructor(opts = {}) {
    this.o = { ...DEFAULTS, ...opts };
    this.capacity = 30 * FPS;
    this.beatLogit = new Float32Array(this.capacity).fill(-100);
    this.downLogit = new Float32Array(this.capacity).fill(-100);
    this.validUntil = 0;  // ここまでのフレームにロジットが入っている
    this.confirmedUntil = 0; // ここまでのフレームでビートを確定した
    this.beats = [];      // 確定したビートの時刻
    this.downbeats = [];  // 確定したダウンビートの時刻
    this.model = null;    // { period, phase, barLen, downbeat, lastBeat, confidence }
  }

  reset() {
    this.beatLogit.fill(-100);
    this.downLogit.fill(-100);
    this.validUntil = this.confirmedUntil = 0;
    this.beats = [];
    this.downbeats = [];
    this.model = null;
  }

  // 推論結果を取り込む。start は入力の先頭フレームの番号、beat / downbeat は長さ n のロジット。
  addPrediction(start, beat, downbeat) {
    const n = beat.length, cap = this.capacity, end = start + n;
    // 入力の左端 6 フレームは学習時に評価されていない部分なので使わない（公式の推論と同じ）。
    const from = start > 0 ? start + 6 : 0;
    // 前回の結果から間が空いた場合、その間はビートなしとして埋める（リングバッファの古い値を消す）。
    for (let f = Math.max(this.validUntil, end - cap); f < from; f++) {
      this.beatLogit[f % cap] = this.downLogit[f % cap] = -100;
    }
    // 新しい結果ほど右側（未来）の音が見えているので、重なる部分は上書きする。
    for (let f = Math.max(from, end - cap); f < end; f++) {
      this.beatLogit[f % cap] = beat[f - start];
      this.downLogit[f % cap] = downbeat[f - start];
    }
    this.validUntil = end;
    this.confirm(this.validUntil - this.o.guardFrames);
    this.updateModel();
  }

  // フレーム until の手前までのビートを確定する。
  // 前回確定した範囲の終わりの少し手前から探し直して、境目の山も正しく見つける。
  confirm(until) {
    const cap = this.capacity;
    const from = Math.max(this.confirmedUntil - 10, this.validUntil - cap + 3, 3);
    if (until <= from) return;

    const pick = (logit, out) => {
      // 探し直す範囲にある確定済みのビートは、新しい結果で置き換える。
      while (out.length && out.at(-1) * FPS >= from) out.pop();
      let last = -Infinity;
      for (let f = from; f < until; f++) {
        const v = logit[f % cap];
        if (v <= 0) continue;
        // ±3 フレーム（±60ms）の中で最大なら山とみなす（公式は max_pool1d(7) で同じことをしている）
        let isMax = true;
        for (let d = -3; d <= 3 && isMax; d++) if (d && logit[(f + d) % cap] > v) isMax = false;
        if (!isMax) continue;
        // 隣り合ったフレームが同じ値で並んだときは 1 つにまとめる（公式の deduplicate_peaks と同じ）
        if (f - last <= 1) { out[out.length - 1] = (last + f) / 2 / FPS; last = f; continue; }
        // 境目の直前に残したビートと同じ山を二重に数えない
        if (out.length && f / FPS - out.at(-1) < 4 / FPS) continue;
        out.push(f / FPS);
        last = f;
      }
    };
    pick(this.beatLogit, this.beats);
    pick(this.downLogit, this.downbeats);
    this.confirmedUntil = until;

    // 古い記録を捨てる
    const keepFrom = this.confirmedUntil / FPS - 20;
    while (this.beats.length && this.beats[0] < keepFrom) this.beats.shift();
    while (this.downbeats.length && this.downbeats[0] < keepFrom) this.downbeats.shift();
  }

  updateModel() {
    const o = this.o;
    const now = this.confirmedUntil / FPS;
    const recent = this.beats.filter((t) => t > now - o.historySec);
    const prev = this.model;
    if (recent.length < o.minBeats) { this.model = null; return; }

    // テンポの候補: 間隔の中央値（全体と直近）と、前回のテンポおよびその 2 倍・半分。
    // モデルは曲によって「拍」と「半拍」のどちらを拍とみなすかが揺れることがあるので、
    // 前回の値とその倍・半分も候補に入れ、前回に近いものを少し優遇して切り替わりを抑える。
    const ibis = [];
    for (let i = 1; i < recent.length; i++) ibis.push(recent[i] - recent[i - 1]);
    const cands = [median(ibis), median(ibis.slice(-4))];
    if (prev) cands.push(prev.period, prev.period * 2, prev.period / 2);

    let best = null;
    for (const p of cands) {
      if (!(p >= o.minPeriod && p <= o.maxPeriod)) continue;
      const fit = fitGrid(recent, p, now);
      if (!fit) continue;
      if (prev && Math.abs(fit.period / prev.period - 1) < 0.08) fit.score += 1;
      if (!best || fit.score > best.score) best = fit;
    }
    if (!best) { this.model = null; return; }
    this.model = { ...best, lastBeat: recent.at(-1), downbeat: this.estimateBar(best.period) };
  }

  // 小節の頭と拍数を推定する。最近のダウンビートを、確定ビートの格子に当てはめる。
  estimateBar(period) {
    const now = this.confirmedUntil / FPS;
    const recent = this.downbeats.filter((t) => t > now - 12);
    if (recent.length < 2) return null;
    const counts = [];
    for (let i = 1; i < recent.length; i++) {
      const n = Math.round((recent[i] - recent[i - 1]) / period);
      if (n >= 2 && n <= 7) counts.push(n);
    }
    if (!counts.length) return null;
    return { barLen: mode(counts), last: recent.at(-1) };
  }

  // after より後に来る最初の予測ビートを返す。予測できないときは null。
  // { time, downbeat, confidence } の confidence は 0〜1（テンポが安定しているほど大きい）。
  next(after) {
    const m = this.model;
    if (!m) return null;
    const lastConfirmed = this.confirmedUntil / FPS;
    let k = Math.floor((after - m.phase) / m.period) + 1;
    // 丸め誤差で after と同じ時刻が返ると、呼び出し側が同じビートを繰り返し受け取ってしまう
    if (m.phase + k * m.period <= after + 1e-6) k++;
    const time = m.phase + k * m.period;
    // 曲が止まった・テンポが崩れたなどで、しばらくビートを確定できていなければ予測しない
    if (lastConfirmed - m.lastBeat > this.o.maxGapBeats * m.period) return null;
    // 確定した範囲から遠すぎる先（推論が止まっている場合など）は予測しない
    if (time - lastConfirmed > 2) return null;
    let downbeat = false;
    const bar = m.downbeat;
    if (bar) {
      const n = Math.round((time - bar.last) / m.period);
      downbeat = ((n % bar.barLen) + bar.barLen) % bar.barLen === 0;
    }
    const confidence = Math.max(0, Math.min(1, 1 - m.jitter / (m.period * 0.15)));
    return { time, downbeat, confidence, period: m.period };
  }

  // 画面表示用: 現在のテンポ (BPM)
  get bpm() {
    return this.model ? 60 / this.model.period : 0;
  }
}

// ビートの時刻 beats を、間隔 period の格子 phase + k*period に当てはめる。
// 格子の基準には直近のビートを順に試し、いちばん点数の高いものを選ぶ。
//   点数 = 格子に乗ったビートの数 − 0.5 × ビートのない格子点の数（どちらも新しいほど重い）
// 選んだ格子に乗ったビートで、間隔と位相を最小二乗で求め直す。
function fitGrid(beats, period, now) {
  const tol = period * 0.15;
  const weight = (t) => Math.exp((t - now) / 3);
  let best = null;
  for (const anchor of beats.slice(-8)) {
    const ks = [], ts = [];
    let score = 0;
    const hit = new Set();
    for (const t of beats) {
      const k = Math.round((t - anchor) / period);
      if (Math.abs(t - (anchor + k * period)) < tol && !hit.has(k)) {
        ks.push(k); ts.push(t); hit.add(k);
        score += weight(t);
      }
    }
    // ビートのない格子点（最初のビートから、確定済みの範囲の終わりまで）
    const kFirst = Math.round((beats[0] - anchor) / period);
    const kLast = Math.floor((now - anchor) / period);
    for (let k = kFirst; k <= kLast; k++) if (!hit.has(k)) score -= 0.5 * weight(anchor + k * period);
    if (ks.length < 3) continue;
    if (best && score <= best.score) continue;
    const fit = linearFit(ks, ts);
    const p = Math.abs(fit.slope / period - 1) < 0.1 ? fit.slope : period;
    const phase = Math.abs(fit.slope / period - 1) < 0.1 ? fit.intercept : anchor;
    // 予測のばらつき（当てはめの残差）を、確からしさの目安にする。
    let sq = 0;
    for (let i = 0; i < ks.length; i++) sq += (ts[i] - (phase + ks[i] * p)) ** 2;
    best = { period: p, phase, score, jitter: Math.sqrt(sq / ks.length) };
  }
  return best;
}

function median(a) {
  const s = [...a].sort((x, y) => x - y);
  const m = s.length >> 1;
  return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2;
}

function mode(a) {
  const c = new Map();
  for (const v of a) c.set(v, (c.get(v) || 0) + 1);
  let best = a[0], n = 0;
  for (const [v, k] of c) if (k > n || (k === n && v === 4)) { best = v; n = k; }
  return best;
}

function linearFit(xs, ys) {
  const n = xs.length;
  let sx = 0, sy = 0, sxx = 0, sxy = 0;
  for (let i = 0; i < n; i++) { sx += xs[i]; sy += ys[i]; sxx += xs[i] * xs[i]; sxy += xs[i] * ys[i]; }
  const d = n * sxx - sx * sx;
  if (d === 0) return { slope: 0, intercept: sy / n };
  const slope = (n * sxy - sx * sy) / d;
  return { slope, intercept: (sy - slope * sx) / n };
}
