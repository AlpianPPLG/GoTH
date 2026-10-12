# Tambahkan Go dan templ ke PATH sesi ini
$env:PATH = "C:\Program Files\Go\bin;$env:USERPROFILE\go\bin;" + $env:PATH

Write-Host "======================================================" -ForegroundColor Cyan
Write-Host " ⚡ PulseOps - Real-Time GoTH Dashboard" -ForegroundColor Cyan
Write-Host " Stack: Go + Templ + HTMX (SSE + OOB Swaps) + Tailwind" -ForegroundColor Gray
Write-Host "======================================================" -ForegroundColor Cyan

# Generate template jika ada perubahan
Write-Host "`n[1/2] Generating Templ components..." -ForegroundColor Yellow
templ generate

# Jalankan server
Write-Host "[2/2] Starting server at http://localhost:8080..." -ForegroundColor Green
go run ./cmd/server
