$ErrorActionPreference = "Stop"

$API_ESTOQUE = "http://localhost:5001/api/v1"
$API_FATURAMENTO = "http://localhost:5002/api/v1"

function Chamar-Api {
    param([string]$Metodo, [string]$Url, [object]$Corpo = $null, [hashtable]$Headers = @{})
    
    $params = @{ Method = $Metodo; Uri = $Url; Headers = $Headers; ContentType = "application/json" }
    if ($Corpo) { $params.Body = ($Corpo | ConvertTo-Json -Depth 10) }
    
    try { 
        return Invoke-RestMethod @params 
    }
    catch { 
        Write-Host "Erro: $_" -ForegroundColor Red
        return $null
    }
}

Write-Host "`n=== DEMO Sistema NFe - Viasoft Korp ===`n" -ForegroundColor Cyan

Write-Host "Aguardando servicos iniciarem..." -ForegroundColor Yellow
Start-Sleep -Seconds 5

# Cenario 1: Fluxo Feliz
Write-Host "`n--- Cenario 1: Fluxo Normal (Reserva + Impressao) ---`n" -ForegroundColor Green

$prod1 = Chamar-Api POST "$API_ESTOQUE/produtos" @{ sku="DEMO-001"; nome="Produto Demo"; saldo=100 }
if ($prod1) {
    Write-Host "* Produto criado: $($prod1.sku) (saldo inicial: 100)" -ForegroundColor Green
}

$nota1 = Chamar-Api POST "$API_FATURAMENTO/notas" @{ numero="NFE-001" }
if ($nota1) {
    Write-Host "* Nota fiscal criada: $($nota1.numero)" -ForegroundColor Green
}

if ($prod1 -and $nota1) {
    Chamar-Api POST "$API_FATURAMENTO/notas/$($nota1.id)/itens" @{ 
        produtoId=$prod1.id; quantidade=30; precoUnitario=15.50 
    } | Out-Null
    Write-Host "* Item adicionado (quantidade: 30)" -ForegroundColor Green

    $chave1 = [guid]::NewGuid().ToString()
    $imp1 = Chamar-Api POST "$API_FATURAMENTO/notas/$($nota1.id)/imprimir" -Headers @{
        "Idempotency-Key"=$chave1
    }
    
    if ($imp1) {
        Write-Host "* Impressao iniciada (ID: $($imp1.id)), aguardando..." -NoNewline -ForegroundColor Cyan

        $maxTentativas = 30
        $tentativa = 0
        $concluida = $false

        while ($tentativa -lt $maxTentativas) {
            Start-Sleep -Milliseconds 1000
            $st1 = Chamar-Api GET "$API_FATURAMENTO/solicitacoes-impressao/$($imp1.id)"
            
            if ($st1.status -ne "PENDENTE") {
                Write-Host ""
                if ($st1.status -eq "CONCLUIDA") {
                    Write-Host "* Nota impressa com sucesso!" -ForegroundColor Green
                    $concluida = $true
                } else {
                    Write-Host "X Falhou: $($st1.mensagemErro)" -ForegroundColor Red
                }
                break
            }
            
            Write-Host "." -NoNewline
            $tentativa++
        }

        if (-not $concluida -and $tentativa -ge $maxTentativas) {
            Write-Host "`n! Timeout aguardando conclusao" -ForegroundColor Yellow
        }

        Start-Sleep -Seconds 2
        $prodFinal = Chamar-Api GET "$API_ESTOQUE/produtos/$($prod1.id)"
        if ($prodFinal) {
            $esperado = 70
            if ($prodFinal.saldo -eq $esperado) {
                Write-Host "* Saldo atualizado corretamente: $($prodFinal.saldo) (esperado: $esperado)" -ForegroundColor Green
            } else {
                Write-Host "! Saldo incorreto: $($prodFinal.saldo) (esperado: $esperado)" -ForegroundColor Yellow
            }
        }
    }
}

# Cenario 2: Saldo Insuficiente
Write-Host "`n--- Cenario 2: Saldo Insuficiente ---`n" -ForegroundColor Yellow

$prod2 = Chamar-Api POST "$API_ESTOQUE/produtos" @{ sku="DEMO-002"; nome="Produto Limitado"; saldo=10 }
if ($prod2) {
    Write-Host "* Produto criado com saldo baixo: $($prod2.saldo)" -ForegroundColor Yellow
}

$nota2 = Chamar-Api POST "$API_FATURAMENTO/notas" @{ numero="NFE-002" }
if ($nota2 -and $prod2) {
    Chamar-Api POST "$API_FATURAMENTO/notas/$($nota2.id)/itens" @{ 
        produtoId=$prod2.id; quantidade=50; precoUnitario=25 
    } | Out-Null
    Write-Host "* Item adicionado (quantidade solicitada: 50 > saldo: 10)" -ForegroundColor Yellow

    $chave2 = [guid]::NewGuid().ToString()
    $imp2 = Chamar-Api POST "$API_FATURAMENTO/notas/$($nota2.id)/imprimir" -Headers @{
        "Idempotency-Key"=$chave2
    }

    if ($imp2) {
        Write-Host "* Impressao iniciada, aguardando rejeicao..." -NoNewline

        Start-Sleep -Seconds 5
        $st2 = Chamar-Api GET "$API_FATURAMENTO/solicitacoes-impressao/$($imp2.id)"
        
        Write-Host ""
        if ($st2.status -eq "FALHOU") {
            Write-Host "* Rejeicao detectada: $($st2.mensagemErro)" -ForegroundColor Green
        } else {
            Write-Host "! Status inesperado: $($st2.status)" -ForegroundColor Yellow
        }
    }
}

# Cenario 3: Idempotencia
Write-Host "`n--- Cenario 3: Idempotencia (mesma chave 2x) ---`n" -ForegroundColor Magenta

$prod3 = Chamar-Api POST "$API_ESTOQUE/produtos" @{ sku="DEMO-003"; nome="Produto Concorrencia"; saldo=40 }
$nota3 = Chamar-Api POST "$API_FATURAMENTO/notas" @{ numero="NFE-003" }

if ($prod3 -and $nota3) {
    Chamar-Api POST "$API_FATURAMENTO/notas/$($nota3.id)/itens" @{ 
        produtoId=$prod3.id; quantidade=20; precoUnitario=10 
    } | Out-Null

    $chaveDuplicada = [guid]::NewGuid().ToString()
    Write-Host "Disparando 2 requisicoes com MESMA chave de idempotencia..." -ForegroundColor Magenta

    $resp1 = Chamar-Api POST "$API_FATURAMENTO/notas/$($nota3.id)/imprimir" -Headers @{
        "Idempotency-Key"=$chaveDuplicada
    }

    Start-Sleep -Milliseconds 500
    $resp2 = Chamar-Api POST "$API_FATURAMENTO/notas/$($nota3.id)/imprimir" -Headers @{
        "Idempotency-Key"=$chaveDuplicada
    }

    if ($resp1 -and $resp2) {
        if ($resp1.id -eq $resp2.id) {
            Write-Host "* Idempotencia OK - Ambas retornaram mesmo ID: $($resp1.id)" -ForegroundColor Green
        } else {
            Write-Host "X Idempotencia FALHOU - IDs diferentes: $($resp1.id) vs $($resp2.id)" -ForegroundColor Red
        }
    }
}

# Cenario 4: Rollback com X-Demo-Fail
Write-Host "`n--- Cenario 4: Rollback com X-Demo-Fail ---`n" -ForegroundColor Red

$prod4 = Chamar-Api POST "$API_ESTOQUE/produtos" @{ sku="DEMO-004"; nome="Produto Rollback"; saldo=50 }
if ($prod4) {
    Write-Host "* Produto criado (saldo: 50)" -ForegroundColor Red

    Write-Host "Tentando reserva com header X-Demo-Fail..." -ForegroundColor Red
    
    $nota4 = Chamar-Api POST "$API_FATURAMENTO/notas" @{ numero="NFE-004" }
    if ($nota4) {
        $result = Chamar-Api POST "$API_ESTOQUE/reservas" @{ 
            notaId=$nota4.id; produtoId=$prod4.id; quantidade=20 
        } -Headers @{ "X-Demo-Fail"="true" }

        Start-Sleep -Seconds 2
        
        $prod4Final = Chamar-Api GET "$API_ESTOQUE/produtos/$($prod4.id)"
        if ($prod4Final.saldo -eq 50) {
            Write-Host "* Rollback OK - Saldo permanece: 50" -ForegroundColor Green
        } else {
            Write-Host "X Rollback FALHOU - Saldo: $($prod4Final.saldo)" -ForegroundColor Red
        }
    }
}

Write-Host "`n=== Demo Concluida ===`n" -ForegroundColor Cyan
Write-Host "Resumo dos cenarios testados:" -ForegroundColor White
Write-Host "1. Fluxo feliz: Reserva -> Impressao -> Baixa estoque" -ForegroundColor Green
Write-Host "2. Saldo insuficiente: Rejeicao detectada" -ForegroundColor Yellow
Write-Host "3. Idempotencia: Mesma chave retorna mesmo resultado" -ForegroundColor Magenta
Write-Host "4. Rollback: X-Demo-Fail nao persiste mudancas" -ForegroundColor Red
Write-Host ""
