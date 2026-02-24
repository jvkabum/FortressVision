# Script de Compilação FortressVision (Modular - MSYS2 Static)
$env:PATH = "C:\msys64\mingw64\bin;" + $env:PATH
$env:CC = "gcc"

Write-Host "Iniciando compilação profissional (Static Link)..." -ForegroundColor Cyan

# 1. Compilar Servidor
Write-Host "`n[1/3] Compilando SERVIDOR (CGO + Static)..." -ForegroundColor Yellow
$env:CGO_ENABLED=1
go build -ldflags="-extldflags=-static -s -w" -o servidor/server.exe ./servidor
if ($LASTEXITCODE -eq 0) { 
    Write-Host "Servidor compilado com sucesso!" -ForegroundColor Green 
} else {
    Write-Host "ERRO na compilação do servidor." -ForegroundColor Red
    pause; exit 1
}

# 2. Compilar Cliente
Write-Host "`n[2/3] Compilando CLIENTE (CGO + Static + GUI)..." -ForegroundColor Yellow
go build -ldflags="-extldflags=-static -s -w -H=windowsgui" -o cliente/client.exe ./cliente
if ($LASTEXITCODE -eq 0) { 
    Write-Host "Cliente compilado com sucesso!" -ForegroundColor Green 
} else {
    Write-Host "ERRO na compilação do cliente." -ForegroundColor Red
    pause; exit 1
}

# 3. Compilar Launcher
Write-Host "`n[3/3] Compilando LAUNCHER (Pure Go)..." -ForegroundColor Yellow
$env:CGO_ENABLED=0
go build -ldflags="-s -w" -o FortressVision.exe ./launcher
if ($LASTEXITCODE -eq 0) { 
    Write-Host "Launcher compilado com sucesso!" -ForegroundColor Green 
} else {
    Write-Host "ERRO na compilação do launcher." -ForegroundColor Red
    pause; exit 1
}

Write-Host "`nBuild finalizada com sucesso!" -ForegroundColor Cyan
Write-Host "Dica: Execute o 'FortressVision.exe' para jogar." -ForegroundColor Gray
pause
