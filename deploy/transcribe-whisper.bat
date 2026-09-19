@echo off
rem TEKSES_TRANSCRIBER bu dosyaya isaret eder; isi PowerShell betigi yapar.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0transcribe-whisper.ps1" %1
