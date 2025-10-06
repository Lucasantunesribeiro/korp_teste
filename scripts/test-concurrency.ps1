Clear-Host
Write-Host "=== TESTE CONCORRÊNCIA - RESERVA DE ESTOQUE ===" -ForegroundColor Cyan

$estoqueApi = "http://localhost:5001/api/v1"

try {
    $produto = Invoke-RestMethod -Method Post -Uri "$estoqueApi/produtos" -ContentType "application/json" -Body (@{
        sku   = "CONC-" + (Get-Date -Format "HHmmss")
        nome  = "Produto Concurrency"
        saldo = 5
    } | ConvertTo-Json)
}
catch {
    Write-Host "Falha ao criar produto base: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host "Produto criado: $($produto.sku) | Saldo inicial: $($produto.saldo)" -ForegroundColor Yellow
$notaId = [guid]::NewGuid().ToString()
$body   = @{ notaId = $notaId; produtoId = $produto.id; quantidade = 3 } | ConvertTo-Json

Write-Host "`nDisparando duas reservas concorrentes (quantidade=3) para o mesmo produto..." -ForegroundColor Yellow

$scriptBlock = {
    param($url, $body)
    try {
        $resp = Invoke-RestMethod -Method Post -Uri $url -Body $body -ContentType "application/json" -ErrorAction Stop
        [pscustomobject]@{
            Sucesso    = $true
            StatusCode = 200
            Conteudo   = $resp
        }
    }
    catch {
        $webResp = $_.Exception.Response
        $status  = $webResp.StatusCode.value__
        $reader  = New-Object System.IO.StreamReader($webResp.GetResponseStream())
        $content = $reader.ReadToEnd()
        [pscustomobject]@{
            Sucesso    = $false
            StatusCode = $status
            Conteudo   = $content
        }
    }
}

$jobs = 1..2 | ForEach-Object {
    Start-Job -ScriptBlock $scriptBlock -ArgumentList "$estoqueApi/reservas", $body
}

Wait-Job -Job $jobs | Out-Null
$results = $jobs | ForEach-Object { Receive-Job $_ }
$jobs | Remove-Job

foreach ($result in $results) {
    $outcome = if ($result.Sucesso) { "SUCESSO" } else { "FALHA" }
    $color   = if ($result.Sucesso) { "Green" } else { "Red" }
    Write-Host "  -> $outcome | Status $($result.StatusCode)" -ForegroundColor $color
}

$produtoFinal = Invoke-RestMethod -Uri "$estoqueApi/produtos/$($produto.id)"
$saldoFinal   = [int]$produtoFinal.saldo

Write-Host "`nSaldo final do produto: $saldoFinal" -ForegroundColor Cyan

if ($saldoFinal -eq 2) {
    Write-Host "CONCORRÊNCIA OK: apenas uma reserva concluiu, a outra foi rejeitada." -ForegroundColor Green
} else {
    Write-Host "ATENÇÃO: saldo final inesperado. Verificar logs para entender o comportamento." -ForegroundColor Red
}
