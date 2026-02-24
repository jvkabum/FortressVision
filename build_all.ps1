# Script de Compilação FortressVision (Modular)
$env:PATH = "C:\TDM-GCC-64\bin;C:\TDM-GCC-32\bin;" + $env:PATH
$env:CC = "gcc"

Write-Host "Iniciando compilação modular..." -ForegroundColor Cyan

# 1. Compilar Servidor
Write-Host "`n[1/2] Compilando SERVIDOR..." -ForegroundColor Yellow
go build -o servidor/server.exe ./servidor
if ($LASTEXITCODE -eq 0) { Write-Host "Servidor compilado com sucesso em servidor/server.exe" -ForegroundColor Green }

# 2. Compilar Cliente
Write-Host "`n[2/2] Compilando CLIENTE..." -ForegroundColor Yellow
go build -o cliente/client.exe ./cliente
if ($LASTEXITCODE -eq 0) { Write-Host "Cliente compilado com sucesso em cliente/client.exe" -ForegroundColor Green }

Write-Host "`nCompilação finalizada." -ForegroundColor Cyan
pause
