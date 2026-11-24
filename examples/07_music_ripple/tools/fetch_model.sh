#!/bin/sh
# Beat This! の学習済みモデルをダウンロードして、ブラウザ用の ONNX に変換します。
# すでに変換済みのファイルがあれば何もしません。
#
#   sh fetch_model.sh [small0|final0]
#
# 初回は、このディレクトリの .venv に PyTorch などをインストールします（数百 MB）。
# チェックポイントは beat_this が JKU のサーバーからダウンロードします
# （~/.cache/torch/hub/checkpoints に保存されます）。
set -eu

MODEL=${1:-small0}
TOOLS=$(cd "$(dirname "$0")" && pwd)
OUT="$TOOLS/../web/models/beat_this_$MODEL.onnx"
VENV="$TOOLS/.venv"

if [ -f "$OUT" ]; then
  exit 0
fi

echo "== Beat This! のモデル $MODEL を用意します（初回のみ）"

if [ ! -x "$VENV/bin/python" ]; then
  # beat_this は Python 3.10 以降が必要。新しいものから順に探す。
  PY=""
  for c in python3.14 python3.13 python3.12 python3.11 python3.10 python3; do
    if command -v "$c" >/dev/null 2>&1 &&
      "$c" -c 'import sys; sys.exit(0 if sys.version_info >= (3, 10) else 1)' 2>/dev/null; then
      PY=$c
      break
    fi
  done
  if [ -z "$PY" ]; then
    echo "Python 3.10 以降が見つかりません（例: brew install python）" >&2
    exit 1
  fi
  echo "== $PY で仮想環境を作り、必要なパッケージをインストールします"
  "$PY" -m venv "$VENV"
  "$VENV/bin/pip" install --quiet --upgrade pip
  "$VENV/bin/pip" install --quiet -r "$TOOLS/requirements.txt"
fi

"$VENV/bin/python" "$TOOLS/export_onnx.py" --model "$MODEL" -o "$OUT.partial.onnx"
mv "$OUT.partial.onnx" "$OUT"
