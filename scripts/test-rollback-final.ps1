Clear-Host
Write-Host "=== TESTE ROLLBACK - VERSÃO FINAL ===" -ForegroundColor Cyan

try {
    $produtos = Invoke-RestMethod -Uri "http://localhost:5001/api/v1/produtos"
    $notas     = Invoke-RestMethod -Uri "http://localhost:5002/api/v1/notas"
} catch {
    Write-Host "Falha ao consultar APIs iniciais: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

if (-not $produtos) {
    Write-Host "ERRO: Nenhum produto encontrado." -ForegroundColor Red
    exit 1
}

if (-not $notas) {
    Write-Host "ERRO: Nenhuma nota encontrada." -ForegroundColor Red
    exit 1
}

$produto = $produtos | Select-Object -First 1
$nota    = $notas    | Select-Object -First 1

Write-Host "`nProduto selecionado:" -ForegroundColor Yellow
Write-Host "  ID:    $($produto.id)"   -ForegroundColor White
Write-Host "  SKU:   $($produto.sku)"  -ForegroundColor White
Write-Host "  Saldo: $($produto.saldo)" -ForegroundColor White

Write-Host "`nNota selecionada:" -ForegroundColor Yellow
Write-Host "  ID:     $($nota.id)"     -ForegroundColor White
Write-Host "  Número: $($nota.numero)" -ForegroundColor White

$saldoAntes = [int]$produto.saldo
Write-Host "`nSALDO ANTES DO TESTE: $saldoAntes" -ForegroundColor Cyan

$body = @{
    notaId     = $nota.id.ToString()
    produtoId  = $produto.id.ToString()
    quantidade = 1
} | ConvertTo-Json

Write-Host "`nBody da requisição:" -ForegroundColor Gray
Write-Host $body -ForegroundColor DarkGray

Write-Host "`nEnviando requisição com X-Demo-Fail=true..." -ForegroundColor Yellow

try {
    Invoke-RestMethod -Method Post `
        -Uri "http://localhost:5001/api/v1/reservas" `
        -Body $body `
        -ContentType "application/json" `
        -Headers @{ "X-Demo-Fail" = "true" } `
        -ErrorAction Stop

    Write-Host "INESPERADO: Requisição retornou sucesso (deveria falhar)" -ForegroundColor Red
} catch {
    $statusCode = $_.Exception.Response.StatusCode.value__
    $statusColor = if ($statusCode -eq 400) { "Green" } else { "Yellow" }
    Write-Host "Status HTTP: $statusCode" -ForegroundColor $statusColor
    Write-Host "Mensagem: $($_.Exception.Message)" -ForegroundColor Gray
}

Write-Host "`nAguardando 3 segundos..." -ForegroundColor Yellow
Start-Sleep -Seconds 3

Write-Host "`nBuscando produto atualizado..." -ForegroundColor Yellow
try {
    $produtoDepois = Invoke-RestMethod -Uri "http://localhost:5001/api/v1/produtos/$($produto.id)"
} catch {
    Write-Host "ERRO: não foi possível consultar o produto após o teste." -ForegroundColor Red
    exit 1
}

$saldoDepois = [int]$produtoDepois.saldo

Write-Host "`n=== RESULTADO ===" -ForegroundColor Cyan
Write-Host "Saldo ANTES:  $saldoAntes"  -ForegroundColor White
Write-Host "Saldo DEPOIS: $saldoDepois" -ForegroundColor White
Write-Host "Diferença:    $($saldoAntes - $saldoDepois)" -ForegroundColor White

if ($saldoAntes -eq $saldoDepois) {
    Write-Host "`nROLLBACK FUNCIONOU!" -ForegroundColor Green
    Write-Host "O saldo não mudou, provando que a transação foi revertida." -ForegroundColor Green
} else {
    Write-Host "`nBUG DETECTADO: ROLLBACK FALHOU!" -ForegroundColor Red
    Write-Host "O saldo foi alterado mesmo com X-Demo-Fail=true." -ForegroundColor Red
    Write-Host "Verificando logs recentes..." -ForegroundColor Yellow

    docker compose logs --tail 30 servico-estoque | Select-String -Pattern "Demo-Fail|Simulação|Reservar"
}
