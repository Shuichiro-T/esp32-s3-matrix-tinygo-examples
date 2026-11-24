// Beat This! の推論を行う Web Worker。
//
// メインスレッドから直近のスペクトログラムを受け取って推論し、結果（ロジット）を返す。
// 推論には onnxruntime-web を使う。WebGPU が使えれば WebGPU、だめなら WASM で動かす。
//
// WASM の推論中はこのスレッドがふさがるので、音声を少しずつ受け取って処理する形にすると
// 処理が追いつかなくなる。そのためスペクトログラムはメインスレッドで作り、
// ここでは「1 回の依頼に 1 回推論して返す」だけにしている。
//
// メッセージ:
//   ← { type: "init", modelUrl, windowFrames, backend: "auto" | "webgpu" | "wasm" }
//   ← { type: "infer", start, n, data }   data: (n * 513) の振幅スペクトル
//   → { type: "ready", backend }
//   → { type: "result", start, beat, downbeat, ms }
//   → { type: "error", message }

import * as ort from "https://cdn.jsdelivr.net/npm/onnxruntime-web@1.31.1/dist/ort.webgpu.min.mjs";
import { N_BINS } from "./dsp.js";

ort.env.wasm.wasmPaths = "https://cdn.jsdelivr.net/npm/onnxruntime-web@1.31.1/dist/";
// マルチスレッドの WASM には特別なヘッダ（COOP/COEP）が必要で、YouTube の埋め込みと両立しないので 1 スレッドで動かす
ort.env.wasm.numThreads = 1;

let session = null;

self.onmessage = async (e) => {
  const msg = e.data;
  try {
    if (msg.type === "init") await init(msg);
    else if (msg.type === "infer") await infer(msg);
  } catch (err) {
    console.error(err);
    self.postMessage({ type: "error", message: err.message || String(err) });
  }
};

async function init({ modelUrl, windowFrames, backend }) {
  const res = await fetch(modelUrl);
  if (!res.ok) throw new Error(`モデルを読み込めません (${res.status} ${modelUrl})`);
  const model = new Uint8Array(await res.arrayBuffer());

  const order = backend === "auto" ? ["webgpu", "wasm"] : [backend];
  let lastErr = null;
  for (const ep of order) {
    if (ep === "webgpu" && !self.navigator?.gpu) {
      lastErr = new Error("このブラウザでは WebGPU を使えません");
      continue;
    }
    try {
      session = await ort.InferenceSession.create(model, { executionProviders: [ep] });
      // 1 回空で動かしておく（WebGPU はシェーダーの準備に時間がかかるため）
      const dummy = new Float32Array(windowFrames * N_BINS);
      await session.run({ spec: new ort.Tensor("float32", dummy, [1, windowFrames, N_BINS]) });
      self.postMessage({ type: "ready", backend: ep });
      return;
    } catch (err) {
      console.warn(ep, err);
      lastErr = err;
      session = null;
    }
  }
  throw lastErr || new Error("推論の実行環境を用意できません");
}

async function infer({ start, n, data }) {
  if (!session) return;
  const t0 = performance.now();
  const out = await session.run({ spec: new ort.Tensor("float32", data, [1, n, N_BINS]) });
  const ms = performance.now() - t0;
  const beat = new Float32Array(out.beat.data);
  const downbeat = new Float32Array(out.downbeat.data);
  self.postMessage({ type: "result", start, beat, downbeat, ms }, [beat.buffer, downbeat.buffer]);
}
