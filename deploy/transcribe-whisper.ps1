# whisper.cpp'yi TekSes söz çıkarma sözleşmesine uyarlar — Windows sürümü.
# (Linux/VM için: transcribe-whisper.sh)
#
# Kurulum (bir kez):
#   1) ffmpeg:  winget install Gyan.FFmpeg   (yeni terminal açın)
#   2) whisper.cpp Windows derlemesi: github.com/ggml-org/whisper.cpp/releases
#      → whisper-bin-x64.zip'i C:\whisper klasörüne açın (whisper-cli.exe)
#   3) Model (~466 MB, çok dilli):
#      https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin
#      → C:\whisper\ggml-small.bin olarak kaydedin
#
# Etkinleştirme (control-api'yi başlatan terminalde, başlatmadan önce):
#   $env:TEKSES_TRANSCRIBER = "C:\dev\TekSesClaude\tekses\deploy\transcribe-whisper.bat"
#
# Çıktı sözleşmesi (transcribe.go): {"segments":[{start_ms,end_ms,text}]}
param([Parameter(Mandatory = $true)][string]$AudioPath)

$ErrorActionPreference = "Stop"

$whisper = if ($env:WHISPER_BIN) { $env:WHISPER_BIN } else { "C:\whisper\whisper-cli.exe" }
$model = if ($env:WHISPER_MODEL) { $env:WHISPER_MODEL } else { "C:\whisper\ggml-small.bin" }
$lang = if ($env:WHISPER_LANG) { $env:WHISPER_LANG } else { "tr" }
# ffmpeg PATH'te değilse FFMPEG_BIN ile tam yol verilebilir.
$ffmpeg = if ($env:FFMPEG_BIN) { $env:FFMPEG_BIN } else { "ffmpeg" }

if (-not (Test-Path $whisper)) { throw "whisper-cli bulunamadı: $whisper (WHISPER_BIN ayarlayın)" }
if (-not (Test-Path $model)) { throw "model bulunamadı: $model (WHISPER_MODEL ayarlayın)" }

# PowerShell 5.1, "Stop" modunda yerel komutların stderr GÜNLÜKLERİNİ bile
# ölümcül hataya çevirir (whisper ilerlemesini stderr'e yazar). Bu yüzden
# yerel çağrılar "Continue" altında koşar; başarı ÇIKIŞ KODUYLA denetlenir,
# günlük yalnızca gerçek başarısızlıkta hataya eklenir.
function Invoke-Native {
    param([string]$Exe, [string[]]$ArgumentList)
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $log = & $Exe @ArgumentList 2>&1
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prev
    if ($code -ne 0) {
        $tail = ($log | ForEach-Object { "$_" } | Select-Object -Last 8) -join "`n"
        throw "$([System.IO.Path]::GetFileName($Exe)) başarısız (çıkış $code): $tail"
    }
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("tekses-tr-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    # Whisper 16 kHz mono WAV ister; mp3/m4a/ogg buradan geçer.
    $wav = Join-Path $tmp "audio.wav"
    Invoke-Native $ffmpeg @("-y", "-loglevel", "error", "-i", $AudioPath, "-ar", "16000", "-ac", "1", $wav)

    Invoke-Native $whisper @("-m", $model, "-l", $lang, "-f", $wav, "-oj", "-of", (Join-Path $tmp "audio"))

    $parsed = Get-Content (Join-Path $tmp "audio.json") -Raw | ConvertFrom-Json
    $segments = @($parsed.transcription | ForEach-Object {
            @{ start_ms = $_.offsets.from; end_ms = $_.offsets.to; text = $_.text }
        })
    # -InputObject: tek öğeli listelerin boru hattında düzleşmesini önler.
    ConvertTo-Json -InputObject @{ segments = $segments } -Depth 4 -Compress
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
