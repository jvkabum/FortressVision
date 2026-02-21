# Script de Compilação FortressVision
# Garante que o compilador C (GCC) seja encontrado pelo Go

# Força os caminhos possíveis do TDM-GCC no PATH desta sessão
$env:PATH = "C:\TDM-GCC-64\bin;C:\TDM-GCC-32\bin;" + $env:PATH
$env:CC = "gcc"

Write-Host "Iniciando compilação do FortressVision-v1..." -ForegroundColor Cyan

# Verifica se o GCC está acessível
if (Get-Command gcc -ErrorAction SilentlyContinue) {
    Write-Host "Compilador detectado com sucesso." -ForegroundColor Gray
} else {
    Write-Host "AVISO: GCC não encontrado no PATH mesmo forçando. Verifique a instalação." -ForegroundColor Red
}

go build -o FortressVision.exe .

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nSucesso! FortressVision.exe gerado." -ForegroundColor Green
    Write-Host "Você já pode rodar o programa." -ForegroundColor Yellow
} else {
    Write-Host "`nErro na compilação. Verifique se o TDM-GCC está instalado em C:\TDM-GCC-64." -ForegroundColor Red
}
pause
