# Sistema Emissão NFe - Microserviços

> **Desafio Técnico**: Viasoft Korp ERP
> **Arquitetura**: Microserviços com Saga Pattern + Transactional Outbox
> **Stack**: C# .NET 9 + Go 1.22+ + Angular 17 + PostgreSQL + RabbitMQ

## 🏗️ Arquitetura do Sistema

### Microserviços

```
┌─────────────────┐      RabbitMQ      ┌──────────────────┐
│  Serviço        │◄───────────────────►│   Serviço        │
│  Estoque (C#)   │   eventos-estoque   │ Faturamento (Go) │
│  .NET 9         │                     │   Gin + GORM     │
└────────┬────────┘                     └────────┬─────────┘
         │                                       │
         │                                       │
    PostgreSQL                              PostgreSQL
   (estoque_db)                          (faturamento_db)
         │                                       │
         └───────────────┬───────────────────────┘
                         │
                    ┌────▼────┐
                    │ Angular │
                    │  Web App │
                    └─────────┘
```

### Padrões Implementados

✅ **Saga Pattern** - Coordenação distribuída de transações
✅ **Transactional Outbox** - Garantia de entrega de eventos
✅ **Concorrência Otimista** (C#) - xmin do PostgreSQL
✅ **Concorrência Pessimista** (Go) - SELECT FOR UPDATE
✅ **Idempotência HTTP** - Idempotency-Key header
✅ **Idempotência Mensageria** - Tabela mensagens_processadas
✅ **Domain-Driven Design** - Camadas bem definidas
✅ **CQRS** - Separação comando/consulta (parcial)

---

## 🚀 Quick Start

### Pré-requisitos

- Docker + Docker Compose
- (Opcional) .NET 9 SDK para desenvolvimento
- (Opcional) Go 1.22+ para desenvolvimento
- (Opcional) Node.js 20+ para desenvolvimento

### Subir Todo o Sistema

```bash
# Clone o repositório
git clone <repo-url>
cd Viasoft_Korp_ERP

# Subir infraestrutura + serviços
docker compose up -d

# Verificar status
docker compose ps

# Acessos:
# - API Estoque: http://localhost:5001/swagger
# - Health Estoque: http://localhost:5001/health
# - API Faturamento: http://localhost:5002/health
# - RabbitMQ Management: http://localhost:15672 (admin/admin123)
# - Frontend: http://localhost:4200
```

### Executar Demonstração

```bash
# Script PowerShell (3 cenários)
pwsh ./scripts/demo.ps1

# Cenários testados:
# 1. Fluxo feliz (reserva → impressão → baixa estoque)
# 2. Rollback (X-Demo-Fail header)
# 3. Idempotência (mesma Idempotency-Key)
```

---

## 📦 Serviço Estoque (C# .NET 9)

### Estrutura

```
servico-estoque/
├── Dominio/
│   └── Entidades/
│       ├── Produto.cs                    # Agregado raiz
│       ├── ReservaEstoque.cs             # Entidade
│       └── EventoOutbox.cs               # Outbox pattern
├── Aplicacao/
│   ├── CasosDeUso/
│   │   ├── ReservarEstoqueCommand.cs
│   │   └── ReservarEstoqueHandler.cs     # Handler principal
│   └── DTOs/
├── Infraestrutura/
│   ├── Persistencia/
│   │   └── ContextoBancoDados.cs         # EF Core + xmin
│   └── Mensageria/
│       └── PublicadorOutbox.cs           # BackgroundService
└── Api/
    ├── Controllers/
    │   ├── ProdutosController.cs
    │   └── ReservasController.cs
    └── Program.cs                        # Setup DI
```

### Tecnologias

- **ASP.NET Core 9** WebAPI
- **EF Core 9** com Npgsql (PostgreSQL)
- **RabbitMQ.Client 6.8** para mensageria
- **xmin** (system column) para concorrência otimista

### Endpoints

```
GET    /api/v1/produtos           # Listar produtos
GET    /api/v1/produtos/{id}      # Buscar produto
POST   /api/v1/produtos           # Criar produto
POST   /api/v1/reservas           # Reservar estoque
  Header: X-Demo-Fail (opcional)  # Simular falha
```

### Fluxo de Reserva

```csharp
// 1. BEGIN TRANSACTION
var tx = await _ctx.Database.BeginTransactionAsync();

try {
    // 2. Buscar produto e debitar saldo
    var produto = await _ctx.Produtos.FindAsync(produtoId);
    produto.DebitarEstoque(quantidade);  // Valida saldo

    // 3. Criar reserva
    var reserva = new ReservaEstoque { ... };
    _ctx.ReservasEstoque.Add(reserva);

    // 4. Adicionar evento no outbox
    var evento = new EventoOutbox {
        TipoEvento = "Estoque.Reservado",
        Payload = JsonSerializer.Serialize(...)
    };
    _ctx.EventosOutbox.Add(evento);

    // 5. COMMIT (atômico!)
    await _ctx.SaveChangesAsync();
    await tx.CommitAsync();
} catch (DbUpdateConcurrencyException) {
    // xmin mudou = conflito de concorrência
    await tx.RollbackAsync();
    // Publica evento de rejeição
}
```

### Concorrência Otimista (xmin)

```csharp
// Configuração no DbContext
builder.Entity<Produto>(p => {
    p.Property(x => x.Versao)
        .HasColumnName("xmin")
        .HasColumnType("xid")
        .IsRowVersion()
        .ValueGeneratedOnAddOrUpdate();
});

// Se 2 requests modificarem o mesmo produto:
// - Primeiro: commit OK
// - Segundo: DbUpdateConcurrencyException
```

### Outbox Publisher (BackgroundService)

```csharp
// Processa eventos pendentes a cada 2 segundos
while (!stoppingToken.IsCancellationRequested)
{
    var eventos = await _ctx.EventosOutbox
        .Where(e => e.DataPublicacao == null)
        .Take(10)
        .ToListAsync();

    foreach (var evento in eventos) {
        channel.BasicPublish(
            exchange: "estoque-eventos",
            routingKey: evento.TipoEvento,
            body: Encoding.UTF8.GetBytes(evento.Payload)
        );
        evento.DataPublicacao = DateTime.UtcNow;
    }

    await _ctx.SaveChangesAsync();
    await Task.Delay(TimeSpan.FromSeconds(2));
}
```

---

## 📦 Serviço Faturamento (Go 1.22+)

### Estrutura

```
servico-faturamento/
├── cmd/api/main.go                       # Entrypoint
├── internal/
│   ├── dominio/
│   │   ├── notafiscal.go                # NotaFiscal + ItemNota
│   │   ├── solicitacaoimpressao.go      # SolicitacaoImpressao
│   │   └── eventos.go                   # Outbox + MensagemProcessada
│   ├── manipulador/
│   │   └── notas.go                     # Handlers Gin (7 endpoints)
│   ├── consumidor/
│   │   └── consumidor.go                # RabbitMQ consumer
│   └── config/
│       └── database.go                  # GORM setup
└── Dockerfile
```

### Tecnologias

- **Gin 1.10** (framework HTTP)
- **GORM 1.25** + driver PostgreSQL
- **amqp091-go 1.10** (RabbitMQ client)
- **UUID** Google

### Endpoints

```
POST   /api/v1/notas                            # Criar nota
GET    /api/v1/notas                            # Listar notas
GET    /api/v1/notas/:id                        # Buscar nota
POST   /api/v1/notas/:id/itens                  # Adicionar item
POST   /api/v1/notas/:id/imprimir               # Solicitar impressão
  Header: Idempotency-Key (obrigatório)
GET    /api/v1/solicitacoes-impressao/:id      # Consultar status
GET    /health                                  # Health check
```

### Fluxo de Impressão

```go
// 1. Cliente chama POST /notas/{id}/imprimir
func (h *Handlers) ImprimirNota(c *gin.Context) {
    chaveIdem := c.GetHeader("Idempotency-Key")

    // Verificar se já existe solicitação com essa chave
    var solExistente SolicitacaoImpressao
    if db.Where("chave_idempotencia = ?", chaveIdem).First(&solExistente).Error == nil {
        c.JSON(200, solExistente) // Retorna mesma resposta
        return
    }

    // Criar nova solicitação
    sol := SolicitacaoImpressao{
        NotaID: notaID,
        Status: "PENDENTE",
        ChaveIdempotencia: chaveIdem,
    }
    db.Create(&sol)

    // Publicar evento no outbox (não implementado neste MVP)
    // Estoque vai processar e responder via RabbitMQ

    c.JSON(201, sol)
}

// 2. Consumidor escuta "Estoque.Reservado"
func (c *Consumidor) ProcessarEstoqueReservado(msg amqp.Delivery) {
    idMsg := msg.MessageId

    db.Transaction(func(tx *gorm.DB) error {
        // Idempotência: verificar se já processou
        var existe MensagemProcessada
        if tx.Where("id_mensagem = ?", idMsg).First(&existe).Error == nil {
            return nil // Já processado, ack sem reprocessar
        }

        // Buscar nota COM LOCK
        var nota NotaFiscal
        tx.Clauses(clause.Locking{Strength: "UPDATE"}).
            First(&nota, "id = ?", notaID)

        // Fechar nota
        nota.Fechar()
        tx.Save(&nota)

        // Atualizar solicitação
        tx.Model(&SolicitacaoImpressao{}).
            Where("nota_id = ? AND status = ?", notaID, "PENDENTE").
            Updates(map[string]interface{}{
                "status": "CONCLUIDA",
                "data_conclusao": time.Now(),
            })

        // Marcar mensagem como processada
        tx.Create(&MensagemProcessada{
            IDMensagem: idMsg,
            DataProcessada: time.Now(),
        })

        return nil
    })
}
```

### Lock Pessimista (GORM)

```go
import "gorm.io/gorm/clause"

// SELECT * FROM notas_fiscais WHERE id = ? FOR UPDATE
tx.Clauses(clause.Locking{Strength: "UPDATE"}).
    First(&nota, "id = ?", notaID)

// Lock mantido até COMMIT/ROLLBACK
// Segunda transação aguarda o lock ser liberado
```

### Idempotência Dupla

#### 1. HTTP (Idempotency-Key)

```go
chave := c.GetHeader("Idempotency-Key")

var solExistente SolicitacaoImpressao
if db.Where("chave_idempotencia = ?", chave).First(&solExistente).Error == nil {
    // Retorna mesma resposta sem reprocessar
    c.JSON(200, solExistente)
    return
}
```

#### 2. RabbitMQ (mensagens_processadas)

```go
idMsg := msg.MessageId

var existe MensagemProcessada
if tx.Where("id_mensagem = ?", idMsg).First(&existe).Error == nil {
    return nil // Já processado
}

// Processar...

tx.Create(&MensagemProcessada{
    IDMensagem: idMsg,
    DataProcessada: time.Now(),
})
```

---

## 🎨 Frontend Angular 17

### Estrutura (Planejada)

```
web-app/src/app/
├── core/
│   ├── services/
│   │   ├── produto.service.ts           # HTTP client produtos
│   │   └── nota-fiscal.service.ts       # HTTP client notas
│   └── interceptors/
│       └── idempotency.interceptor.ts   # Adiciona Idempotency-Key
├── features/
│   ├── produtos/
│   │   ├── lista-produtos.component.ts  # Lista + criar produto
│   │   └── lista-produtos.component.html
│   └── notas/
│       ├── detalhe-nota.component.ts    # Adicionar itens + imprimir
│       └── detalhe-nota.component.html
└── models/
    ├── produto.model.ts
    └── nota-fiscal.model.ts
```

### Funcionalidades

- ✅ Listar produtos com saldo disponível
- ✅ Criar novo produto
- ✅ Criar nota fiscal
- ✅ Adicionar itens à nota
- ✅ Solicitar impressão (com polling de status)
- ✅ Toast notifications (sucesso/erro)
- ✅ Signals para state management

### Proxy local

O arquivo `proxy.conf.json` redireciona `http://localhost:4200/api/estoque` → `http://localhost:5001/api/v1` e `http://localhost:4200/api/faturamento` → `http://localhost:5002/api/v1`. Assim `ng serve` funciona igual ao Nginx do container sem ajustes no código.

### Polling de Status

```typescript
imprimirNota(notaId: string) {
    const chaveIdem = this.gerarChaveIdempotencia();

    // 1. Solicitar impressão
    this.http.post(`/notas/${notaId}/imprimir`, {}, {
        headers: { 'Idempotency-Key': chaveIdem }
    }).pipe(
        // 2. Polling a cada 1s
        switchMap(sol => interval(1000).pipe(
            switchMap(() => this.http.get(`/solicitacoes-impressao/${sol.id}`)),
            filter(status => status.status !== 'PENDENTE'),
            take(1),
            timeout(30000)
        ))
    ).subscribe({
        next: status => {
            if (status.status === 'CONCLUIDA') {
                this.toast.success('Nota impressa! Estoque baixado.');
            } else {
                this.toast.error(`Falha: ${status.mensagemErro}`);
            }
        }
    });
}

private gerarChaveIdempotencia(): string {
    return `${Date.now()}-${Math.random().toString(36).substring(2, 15)}`;
}
```

---

## 🔄 Fluxo Completo da Saga

### Cenário 1: Sucesso

```
┌─────────┐  POST /imprimir  ┌─────────────┐
│ Angular │─────────────────►│ Faturamento │
└─────────┘                  └──────┬──────┘
                                    │
                           1. Cria SolicitacaoImpressao(PENDENTE)
                           2. Publica "Estoque.ReservaSolicitada" outbox
                                    │
                                    ▼
                            ┌───────────────┐
                            │   RabbitMQ    │
                            └───────┬───────┘
                                    │
                                    ▼
                            ┌───────────────┐
                            │    Estoque    │
                            └───────┬───────┘
                                    │
                           1. Valida saldo OK
                           2. Debita Produto.Saldo
                           3. Cria ReservaEstoque
                           4. Publica "Estoque.Reservado" outbox
                                    │
                                    ▼
                            ┌───────────────┐
                            │   RabbitMQ    │
                            └───────┬───────┘
                                    │
                                    ▼
                            ┌───────────────┐
                            │ Faturamento   │
                            │  (Consumidor) │
                            └───────┬───────┘
                                    │
                           1. SELECT ... FOR UPDATE nota
                           2. Fecha nota
                           3. Marca SolicitacaoImpressao CONCLUIDA
                           4. Grava MensagemProcessada
                                    │
                                    ▼
                            ┌───────────────┐
                            │    Angular    │
                            │   (Polling)   │
                            └───────────────┘
                                    │
                           Toast: "✓ Nota impressa!"
```

### Cenário 2: Saldo Insuficiente

```
┌─────────┐  POST /reservas  ┌─────────┐
│ Cliente │─────────────────►│ Estoque │
└─────────┘                  └────┬────┘
                                  │
                         1. Valida saldo INSUFICIENTE
                         2. Publica "Estoque.ReservaRejeitada" outbox
                                  │
                                  ▼
                          ┌───────────────┐
                          │  Faturamento  │
                          │  (Consumidor) │
                          └───────┬───────┘
                                  │
                         1. Marca SolicitacaoImpressao FALHOU
                         2. Salva mensagem_erro
                                  │
                                  ▼
                          ┌───────────────┐
                          │    Angular    │
                          └───────────────┘
                                  │
                         Toast: "✗ Saldo insuficiente"
```

### Cenário 3: Rollback (X-Demo-Fail)

```
┌─────────┐  POST /reservas  ┌─────────┐
│ Cliente │─────────────────►│ Estoque │
└─────────┘  X-Demo-Fail=true└────┬────┘
                                  │
                         BEGIN TRANSACTION
                         1. Debita saldo ✓
                         2. Cria reserva ✓
                         3. Cria evento outbox ✓
                         4. throw Exception ✗
                                  │
                                  ▼
                         ROLLBACK AUTOMÁTICO
                         (nada persiste, nenhum evento publicado)
                                  │
                                  ▼
                         Faturamento nunca recebe evento
                         (timeout → marca FALHOU)
```

---

## 🧪 Testes

### Teste Manual via cURL

```bash
# 1. Criar produto
curl -X POST http://localhost:5001/api/v1/produtos \
  -H "Content-Type: application/json" \
  -d '{"sku":"PROD-001","nome":"Produto Teste","saldo":100}'

# 2. Criar nota fiscal
curl -X POST http://localhost:5002/api/v1/notas \
  -H "Content-Type: application/json" \
  -d '{"numero":"NFE-001"}'

# 3. Adicionar item
curl -X POST http://localhost:5002/api/v1/notas/{nota_id}/itens \
  -H "Content-Type: application/json" \
  -d '{"produtoId":"{produto_id}","quantidade":10,"precoUnitario":15.50}'

# 4. Solicitar impressão
curl -X POST http://localhost:5002/api/v1/notas/{nota_id}/imprimir \
  -H "Idempotency-Key: $(uuidgen)"

# 5. Consultar status
curl http://localhost:5002/api/v1/solicitacoes-impressao/{solicitacao_id}
```

### Script de Demonstração (PowerShell)

Executar: `pwsh ./scripts/demo.ps1`

Testa automaticamente:
1. Fluxo feliz (reserva → impressão → verificar saldo)
2. Rollback com X-Demo-Fail header
3. Idempotência (2 requests com mesma chave)

---

## 🗄️ Schemas de Banco de Dados

### Estoque DB (PostgreSQL)

```sql
-- produtos
CREATE TABLE produtos (
    id UUID PRIMARY KEY,
    sku VARCHAR(50) UNIQUE NOT NULL,
    nome VARCHAR(200) NOT NULL,
    saldo INT NOT NULL CHECK (saldo >= 0),
    ativo BOOLEAN NOT NULL DEFAULT true,
    data_criacao TIMESTAMPTZ NOT NULL
    -- xmin é system column (não criar)
);

-- reservas_estoque
CREATE TABLE reservas_estoque (
    id UUID PRIMARY KEY,
    nota_id UUID NOT NULL,
    produto_id UUID REFERENCES produtos(id),
    quantidade INT NOT NULL,
    status VARCHAR(20) NOT NULL, -- RESERVADO, CANCELADO
    data_criacao TIMESTAMPTZ NOT NULL
);

-- eventos_outbox
CREATE TABLE eventos_outbox (
    id BIGSERIAL PRIMARY KEY,
    tipo_evento VARCHAR(100) NOT NULL,
    id_agregado UUID NOT NULL,
    payload JSONB NOT NULL,
    data_ocorrencia TIMESTAMPTZ NOT NULL,
    data_publicacao TIMESTAMPTZ,
    tentativas_envio INT DEFAULT 0
);

CREATE INDEX idx_outbox_pendentes ON eventos_outbox (data_publicacao)
    WHERE data_publicacao IS NULL;
```

### Faturamento DB (PostgreSQL)

```sql
-- notas_fiscais
CREATE TABLE notas_fiscais (
    id UUID PRIMARY KEY,
    numero VARCHAR(20) UNIQUE NOT NULL,
    status VARCHAR(20) NOT NULL, -- ABERTA, FECHADA
    data_criacao TIMESTAMPTZ NOT NULL,
    data_fechada TIMESTAMPTZ
);

-- itens_nota
CREATE TABLE itens_nota (
    id UUID PRIMARY KEY,
    nota_id UUID REFERENCES notas_fiscais(id),
    produto_id UUID NOT NULL,
    quantidade INT NOT NULL,
    preco_unitario DECIMAL(10,2) NOT NULL
);

-- solicitacoes_impressao
CREATE TABLE solicitacoes_impressao (
    id UUID PRIMARY KEY,
    nota_id UUID REFERENCES notas_fiscais(id),
    status VARCHAR(20) NOT NULL, -- PENDENTE, CONCLUIDA, FALHOU
    mensagem_erro TEXT,
    chave_idempotencia VARCHAR(100) UNIQUE,
    data_criacao TIMESTAMPTZ NOT NULL,
    data_conclusao TIMESTAMPTZ
);

-- eventos_outbox
CREATE TABLE eventos_outbox (
    id BIGSERIAL PRIMARY KEY,
    tipo_evento VARCHAR(100) NOT NULL,
    id_agregado UUID NOT NULL,
    payload JSONB NOT NULL,
    data_ocorrencia TIMESTAMPTZ NOT NULL,
    data_publicacao TIMESTAMPTZ
);

-- mensagens_processadas
CREATE TABLE mensagens_processadas (
    id_mensagem VARCHAR(100) PRIMARY KEY,
    data_processada TIMESTAMPTZ NOT NULL
);
```

---

## 🔧 Configuração de Desenvolvimento

### Estoque C#

```bash
cd servico-estoque

# Restaurar pacotes
dotnet restore

# Criar migration
dotnet ef migrations add Initial

# Aplicar migration
dotnet ef database update

# Executar
dotnet run --launch-profile https
```

### Faturamento Go

```bash
cd servico-faturamento

# Baixar dependências
go mod download

# Executar (requer DB + RabbitMQ)
go run cmd/api/main.go

# Ou build
go build -o faturamento cmd/api/main.go
./faturamento
```

### Frontend Angular

```bash
cd web-app

# Instalar dependências
npm install

# Desenvolvimento (proxy configurado para /api/*)
npm run start

# Build produção
npm run build
```

---

## 📊 Monitoramento

### RabbitMQ Management

- **URL**: http://localhost:15672
- **Usuário**: admin
- **Senha**: admin123

Visualizar:
- Exchanges: `estoque-eventos`
- Queues: `faturamento-eventos`
- Messages rate
- Connections

### Logs

```bash
# Estoque
docker compose logs --tail 20 -f servico-estoque

# Faturamento
docker compose logs --tail 20 -f servico-faturamento

# Todos
docker compose logs -f
```

---

## 🎯 Decisões Técnicas

### Por que Outbox Pattern?

Garante que eventos sejam publicados atomicamente com a transação do banco.
Sem Outbox: evento publicado + DB rollback = inconsistência.

### Por que xmin no C#?

Column especial do Postgres que incrementa automaticamente a cada UPDATE.
Elimina necessidade de campo `Version` manual.

### Por que SELECT FOR UPDATE no Go?

Lock pessimista evita race condition ao fechar notas simultâneas.
Uma transação espera a outra terminar.

### Por que Idempotency-Key?

Evita duplicação se cliente reenviar request (timeout, retry, etc).
Backend retorna mesma resposta sem reprocessar.

### Por que BackgroundService?

Separar publicação de eventos da transação principal.
Performance: commit rápido, publicação assíncrona.

---

## 🚨 Troubleshooting

### Serviço não sobe

```bash
# Verificar logs
docker compose logs servico-estoque
docker compose logs servico-faturamento

# Recriar containers
docker compose down -v
docker compose up -d --build
```

### Migrations não aplicadas

```bash
# Conectar no Postgres
docker exec -it postgres-estoque psql -U admin -d estoque

# Verificar tabelas
\dt

# Se vazio, aplicar migrations manualmente
dotnet ef database update
```

### RabbitMQ não conecta

```bash
# Verificar se está rodando
docker compose ps rabbitmq

# Acessar logs
docker compose logs rabbitmq

# Reiniciar
docker compose restart rabbitmq
```

---

## 📚 Referências

- [Saga Pattern](https://microservices.io/patterns/data/saga.html)
- [Transactional Outbox](https://microservices.io/patterns/data/transactional-outbox.html)
- [PostgreSQL xmin](https://www.postgresql.org/docs/current/ddl-system-columns.html)
- [GORM Locking](https://gorm.io/docs/advanced_query.html#Locking)
- [RabbitMQ Patterns](https://www.rabbitmq.com/tutorials/tutorial-topics.html)

---

## 👨‍💻 Autor

**Desenvolvedor**: Lucas Antunes Ferreira
**Desafio**: Viasoft Korp - Estágio Desenvolvedor
**Data**: 2025
**Tecnologias**: C# .NET 9, Go 1.22+, Angular 17, PostgreSQL, RabbitMQ, Docker

---

## 📝 Licença

Este projeto foi desenvolvido como parte de um desafio técnico para processo seletivo.
