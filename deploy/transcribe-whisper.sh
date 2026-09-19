#!/bin/sh
# whisper.cpp'yi TekSes söz çıkarma sözleşmesine uyarlar (transcribe.go):
# tek argüman ses dosyası; stdout'a {"segments":[{start_ms,end_ms,text}]}.
#
# VM kurulumu (bir kez):
#   sudo apt install -y ffmpeg jq build-essential cmake
#   git clone https://github.com/ggml-org/whisper.cpp /opt/whisper.src
#   cmake -B /opt/whisper.src/build /opt/whisper.src && cmake --build /opt/whisper.src/build -j
#   /opt/whisper.src/models/download-ggml-model.sh small /opt/whisper
#   sudo cp /opt/whisper.src/build/bin/whisper-cli /opt/whisper/
# Sonra control-api ortamına: TEKSES_TRANSCRIBER=/path/to/transcribe-whisper.sh
#
# Not: şarkılarda tanıma hata payı yüksektir (enstrüman vokali bastırır);
# çıktı taslaktır, panelde düzeltilir.
set -eu

AUDIO="$1"
WHISPER="${WHISPER_BIN:-/opt/whisper/whisper-cli}"
MODEL="${WHISPER_MODEL:-/opt/whisper/ggml-small.bin}"
LANG="${WHISPER_LANG:-tr}"

OUT="$(mktemp -d)"
trap 'rm -rf "$OUT"' EXIT

# Whisper 16 kHz mono WAV ister; mp3/m4a/ogg buradan geçer.
ffmpeg -y -loglevel error -i "$AUDIO" -ar 16000 -ac 1 "$OUT/audio.wav"

"$WHISPER" -m "$MODEL" -l "$LANG" -f "$OUT/audio.wav" -oj -of "$OUT/audio" >/dev/null

# whisper.cpp JSON'u → tekses sözleşmesi
jq '{segments: [.transcription[] | {start_ms: .offsets.from, end_ms: .offsets.to, text: .text}]}' \
  "$OUT/audio.json"
