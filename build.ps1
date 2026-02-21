# Script de Compilação FortressVision
# Garante que o compilador C (GCC) seja encontrado pelo Go

# Força o caminho do GCC no PATH desta sessão
$env:PATH = "C:\TDM-GCC-64\bin;" + $env:PATH
$env:CC = "gcc"

Write-Host "Iniciando compilação do FortressVision-v1..." -ForegroundColor Cyan
Write-Host "Usando compilador em: C:\TDM-GCC-64\bin\gcc.exe" -ForegroundColor Gray

go build -o FortressVision.exe .

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nSucesso! FortressVision.exe gerado." -ForegroundColor Green
    Write-Host "Você já pode rodar o programa." -ForegroundColor Yellow
} else {
    Write-Host "`nErro na compilação. Verifique se o TDM-GCC está em C:\TDM-GCC-64." -ForegroundColor Red
}
pause
