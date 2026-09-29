@echo off
rem Build lalu jalankan dashboard di http://localhost:8080
rem Hanya bisa dibuka dari laptop ini, jadi Windows Firewall tidak bertanya.
rem Kalau mau dibuka dari HP di WiFi yang sama, ganti localhost:8080 jadi :8080
cd /d "%~dp0"

go build -o jira-weekly.exe . || (pause & exit /b 1)

start "" cmd /c "timeout /t 3 >nul & start http://localhost:8080"
jira-weekly.exe -web localhost:8080 -interval 1h
pause
