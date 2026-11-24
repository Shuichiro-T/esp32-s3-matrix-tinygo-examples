"""Beat This! の学習済みモデルを、ブラウザで動かせる ONNX 形式に変換します。

    python export_onnx.py                 # small0 → ../web/models/beat_this_small0.onnx
    python export_onnx.py --model final0  # 大きいモデル（約 80MB、WebGPU 推奨）

ふだんは直接実行せず、fetch_model.sh（make serve / make model から呼ばれる）を使ってください。
仮想環境の用意からまとめて行います。直接実行する場合は Python 3.10 以降と
requirements.txt のパッケージが必要です。

ONNX の入力と出力:
    入力  spec   (1, T, 513) float32: 22050Hz の音を 1024 点の周期ハン窓で切り出し、
                                       rfft した振幅 |X|（正規化なし）。1 フレーム = 441 サンプル (20ms)。
    出力  beat     (1, T) float32: ビートらしさのロジット（> 0 でビート）
          downbeat (1, T) float32: 小節の頭らしさのロジット

beat_this の LogMelSpect（torchaudio の MelSpectrogram）のうち、
STFT 以降の「正規化 → mel フィルタバンク → log1p(1000x)」をモデルの先頭に組み込んでいます。
ブラウザ側は FFT の振幅だけを計算すればよくなります。
"""

import argparse
from pathlib import Path

import torch
import torchaudio.functional as AF

from beat_this.inference import load_model
from beat_this.model import roformer
from beat_this.model.beat_tracker import SumHead

SAMPLE_RATE = 22050
N_FFT = 1024
N_BINS = N_FFT // 2 + 1


class PlainRotary(torch.nn.Module):
    """rotary_embedding_torch.RotaryEmbedding.rotate_queries_or_keys と同じ計算をする置き換え。

    元の実装は @autocast デコレータと内部キャッシュ（forward 中の状態の書き換え）を使っていて、
    ONNX に変換できないため。
    """

    def __init__(self, freqs):
        super().__init__()
        self.register_buffer("freqs", freqs.detach().clone())

    def rotate_queries_or_keys(self, t):
        # t: (batch, heads, seq, dim_head)
        pos = torch.arange(t.shape[-2], device=t.device, dtype=self.freqs.dtype)
        f = (pos[:, None] * self.freqs[None, :]).repeat_interleave(2, dim=-1)
        x = t.unflatten(-1, (-1, 2))
        rot = torch.stack((-x[..., 1], x[..., 0]), dim=-1).flatten(-2)
        return t * f.cos() + rot * f.sin()


def replace_rotary(model):
    for m in model.modules():
        if isinstance(m, roformer.Attention) and m.rotary_embed is not None:
            m.rotary_embed = PlainRotary(m.rotary_embed.freqs)


class SpectToBeats(torch.nn.Module):
    def __init__(self, model):
        super().__init__()
        self.model = model
        self.sum_head = isinstance(model.task_heads, SumHead)
        # preprocessing.LogMelSpect と同じ設定（norm=None の slaney メル）
        fb = AF.melscale_fbanks(
            n_freqs=N_BINS,
            f_min=30.0,
            f_max=11000.0,
            n_mels=128,
            sample_rate=SAMPLE_RATE,
            norm=None,
            mel_scale="slaney",
        )
        # normalized="frame_length" は torch.stft(normalized=True)、つまり 1/sqrt(n_fft) 倍
        self.register_buffer("fb", fb / N_FFT**0.5)

    def forward(self, spec):
        mel = torch.log1p(1000 * (spec @ self.fb))
        x = self.model.frontend(mel)
        x = self.model.transformer_blocks(x)
        # SumHead と同じ計算。SumHead の中の torch.autocast は ONNX に変換できないので展開する。
        bd = self.model.task_heads.beat_downbeat_lin(x)
        beat, downbeat = bd[..., 0], bd[..., 1]
        if self.sum_head:
            beat = beat + downbeat
        return beat, downbeat


def main():
    here = Path(__file__).resolve().parent
    ap = argparse.ArgumentParser()
    ap.add_argument("--model", default="small0", help="beat_this のチェックポイント名（small0, final0 など）")
    ap.add_argument("--frames", type=int, default=500, help="変換時に使うダミー入力のフレーム数")
    ap.add_argument("-o", "--output", type=Path, help="出力先（省略時は ../web/models/beat_this_<model>.onnx）")
    args = ap.parse_args()

    out = args.output or here.parent / "web" / "models" / f"beat_this_{args.model}.onnx"
    out.parent.mkdir(parents=True, exist_ok=True)

    model = load_model(args.model, "cpu")
    wrapper = SpectToBeats(model).eval()
    dummy = torch.rand(1, args.frames, N_BINS) * 10
    with torch.no_grad():
        expected = model(torch.log1p(1000 * (dummy @ wrapper.fb)))
        replace_rotary(model)
        got = wrapper(dummy)
    diff = max((expected["beat"] - got[0]).abs().max(), (expected["downbeat"] - got[1]).abs().max())
    print(f"置き換え前後の出力の差: {diff:.2e}")
    assert diff < 1e-3, "置き換えたモデルの出力が元のモデルと一致しません"

    time = torch.export.Dim("time", min=16, max=1500)
    torch.onnx.export(
        wrapper,
        (dummy,),
        out,
        input_names=["spec"],
        output_names=["beat", "downbeat"],
        dynamic_shapes={"spec": {1: time}},
        dynamo=True,
        external_data=False,
        optimize=True,
    )
    print(f"wrote {out} ({out.stat().st_size / 1e6:.1f} MB)")


if __name__ == "__main__":
    main()
