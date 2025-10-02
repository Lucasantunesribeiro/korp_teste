# 🎬 ROTEIRO PARA VÍDEO DE DEMONSTRAÇÃO

## ⏱️ Tempo Total: 20 minutos

---

## 🎯 PARTE 1: INTRODUÇÃO E ARQUITETURA (5 min)

### Slide 1: Apresentação (30s)
```
"Olá! Vou demonstrar um sistema distribuído de emissão de notas fiscais 
implementando Saga Pattern com microserviços em C# e Go."

[Mostrar na tela]
- Nome do projeto: Sistema NFe Microserviços
- Stack: .NET 9, Go 1.23, Angular 17, PostgreSQL, RabbitMQ
- Padrões: Saga, Outbox, Idempotência, CQRS
```

### Slide 2: Visão Geral da Arquitetura (1min)
```
[Abrir ARQUITETURA.md no VS Code]

"O sistema possui 3 componentes principais:

1. FATURAMENTO (Go): Gerencia notas fiscais e solicitações de impressão
2. ESTOQUE (C#): Controla produtos e reservas
3. RABBITMQ: Comunicação assíncrona via eventos

A arquitetura foi desenhada para garantir consistência eventual 
sem acoplamento forte entre os serviços."

[Mostrar diagrama ASCII no arquivo]
```

### Slide 3: Saga Pattern Explicada (1min 30s)
```
[Abrir ARQUITETURA.md - seção Saga Pattern]

"Quando o usuário solicita a impressão de uma nota, iniciamos uma SAGA:

1. Faturamento cria SolicitacaoImpressao (status PENDENTE)
2. Publica evento 'Faturamento.ImpressaoSolicitada' no RabbitMQ
3. Estoque consome, valida saldo e debita estoque
4. Estoque publica evento 'Estoque.Reservado'
5. Faturamento consome e fecha a nota
6. Solicitação marcada como CONCLUIDA

Se houver falha, o sistema compensa automaticamente publicando 
eventos de rejeição, sem necessidade de rollback distribuído."
```

### Slide 4: Transactional Outbox (1min)
```
[Mostrar tabela eventos_outbox no código]

"Para garantir atomicidade entre mudança de estado e publicação de evento,
usamos o padrão Transactional Outbox:

1. Mesma transação salva mudança + evento na tabela outbox
2. Worker assíncrono (PublicadorOutbox) lê eventos pendentes
3. Publica no RabbitMQ
4. Marca evento como publicado

Isso resolve o problema do 'dual write' e garante que nenhum evento se perca."

[Mostrar código do PublicadorOutbox.cs ou outbox.go]
```

### Slide 5: Outros Padrões (1min)
```
"Além da Saga, implementei:

✅ IDEMPOTÊNCIA: 
   - HTTP: header Idempotency-Key
   - Mensageria: tabela mensagens_processadas

✅ CONCORRÊNCIA:
   - Otimista (C#): PostgreSQL xmin + EF Core
   - Pessimista (Go): SELECT FOR UPDATE

✅ RESILIÊNCIA:
   - Retry automático no PublicadorOutbox
   - ACK/NACK manual no RabbitMQ
   - Timeouts configurados"
```

---

## 💻 PARTE 2: CÓDIGO E IMPLEMENTAÇÃO (7 min)

### Demo 1: Endpoint de Impressão (2 min)
```
[Abrir servico-faturamento/internal/manipulador/notas.go]

"Vamos ver como o endpoint de impressão funciona:

1. Valida Idempotency-Key (linha X)
2. Inicia transação
3. Cria SolicitacaoImpressao (status PENDENTE)
4. Cria evento no outbox atomicamente
5. Commit da transação

[Destacar código]
func (h *Handlers) ImprimirNota(...) {
    // BEGIN TX
    solicitacao := &SolicitacaoImpressao{
        NotaID: notaID,
        Status: "PENDENTE",
        ChaveIdempotencia: chaveIdem,
    }
    tx.Create(&solicitacao)
    
    // Outbox atomicamente
    evento := &EventoOutbox{
        TipoEvento: "Faturamento.ImpressaoSolicitada",
        Payload: json.Marshal(payload),
    }
    tx.Create(&evento)
    // COMMIT
}

Tudo em uma única transação!"
```

### Demo 2: Publicador Outbox (1 min 30s)
```
[Abrir servico-faturamento/internal/publicador/outbox.go]

"O PublicadorOutbox roda como worker em background:

1. A cada 2 segundos, busca eventos pendentes
2. Publica no RabbitMQ
3. Marca como publicado

[Destacar código]
for {
    eventos := []EventoOutbox{}
    db.Where("data_publicacao IS NULL").
       Limit(10).
       Find(&eventos)
    
    for _, evt := range eventos {
        channel.Publish(..., evt.Payload)
        evt.DataPublicacao = now()
        db.Save(&evt)
    }
    
    time.Sleep(2 * time.Second)
}

Simples e eficiente!"
```

### Demo 3: Consumidor com Idempotência (2 min)
```
[Abrir servico-estoque/Infraestrutura/Mensageria/ConsumidorEventos.cs]

"No Estoque, o consumidor processa eventos de forma idempotente:

[Destacar código - linha 157-164]
// Verificar se já processou
var jaProcessada = await _ctx.MensagensProcessadas
    .AnyAsync(m => m.IDMensagem == idMensagem);

if (jaProcessada) {
    _logger.LogInformation("Mensagem já processada, ignorando");
    return; // ACK sem reprocessar
}

[Linha 220-224]
// Após processar com sucesso, marcar como processada
_ctx.MensagensProcessadas.Add(new MensagemProcessada {
    IDMensagem = idMensagem,
    DataProcessada = DateTime.UtcNow
});
await _ctx.SaveChangesAsync();

Isso garante que mesmo se o RabbitMQ reenviar a mensagem, 
processamos apenas uma vez!"
```

### Demo 4: Concorrência Otimista (1 min 30s)
```
[Abrir servico-estoque/Infraestrutura/Persistencia/ContextoBancoDados.cs]

"Para evitar race conditions ao debitar estoque, uso concorrência otimista:

[Destacar configuração do xmin - linhas 30-34]
p.Property(x => x.Versao)
    .HasColumnName("xmin")
    .HasColumnType("xid")
    .IsRowVersion()
    .ValueGeneratedOnAddOrUpdate();

O PostgreSQL incrementa xmin automaticamente a cada UPDATE.
O EF Core verifica se mudou antes de salvar.

[Abrir ReservarEstoqueHandler.cs - catch DbUpdateConcurrencyException]
Se detectar conflito, publica evento de rejeição:

catch (DbUpdateConcurrencyException) {
    var evt = new EventoOutbox {
        TipoEvento = "Estoque.ReservaRejeitada",
        Payload = "{ motivo: 'Conflito de concorrência' }"
    };
    _ctx.EventosOutbox.Add(evt);
}

Simples e elegante!"
```

---

## 🚀 PARTE 3: DEMONSTRAÇÃO AO VIVO (8 min)

### Setup: Iniciar Ambiente (1 min)
```
[Terminal 1]
$ docker-compose down  # limpar ambiente
$ docker-compose up -d # iniciar tudo

[Aguardar 10s]
$ docker ps  # mostrar containers rodando

[Mostrar]
✓ postgres-estoque
✓ postgres-faturamento  
✓ rabbitmq
✓ servico-estoque
✓ servico-faturamento
✓ web-app
```

### Cenário 1: Fluxo Feliz (3 min)
```
[Postman ou cURL no terminal]

"Vou criar um produto, nota, item e solicitar impressão:"

# 1. Criar produto
POST http://localhost:5001/api/v1/produtos
{
    "sku": "DEMO-001",
    "nome": "Produto Demonstração",
    "saldo": 100
}
→ Resposta: { id: "...", saldo: 100 }

# 2. Criar nota
POST http://localhost:5002/api/v1/notas
{
    "numero": "NFE-DEMO-001"
}
→ Resposta: { id: "...", status: "ABERTA" }

# 3. Adicionar item
POST http://localhost:5002/api/v1/notas/{notaId}/itens
{
    "produtoId": "...",
    "quantidade": 30,
    "precoUnitario": 50.00
}
→ Resposta: { id: "..." }

# 4. Solicitar impressão
POST http://localhost:5002/api/v1/notas/{notaId}/imprimir
Header: Idempotency-Key: demo-live-123
→ Resposta: { solicitacaoId: "...", status: "PENDENTE" }

[Aguardar 2s]

# 5. Consultar status
GET http://localhost:5002/api/v1/solicitacoes-impressao/{id}
→ Resposta: { status: "CONCLUIDA", dataConclusao: "..." }

# 6. Verificar estoque
GET http://localhost:5001/api/v1/produtos/{produtoId}
→ Resposta: { saldo: 70 }  ✓ Debitado 30!

"Sucesso! A Saga processou em ~2 segundos."
```

### Cenário 2: Saldo Insuficiente (2 min)
```
"Agora vou testar o cenário de falha:"

# 1. Criar produto com pouco estoque
POST http://localhost:5001/api/v1/produtos
{
    "sku": "DEMO-LIMITADO",
    "nome": "Produto Limitado",
    "saldo": 5
}

# 2. Criar nota + item com quantidade MAIOR que o saldo
POST http://localhost:5002/api/v1/notas
{ "numero": "NFE-FALHA" }

POST .../itens
{
    "produtoId": "...",
    "quantidade": 10,  # > 5!
    "precoUnitario": 20.00
}

# 3. Tentar imprimir
POST .../imprimir
Header: Idempotency-Key: demo-fail-456

[Aguardar 2s]

GET /solicitacoes-impressao/{id}
→ Resposta: { 
    status: "FALHOU",
    mensagemErro: "Saldo insuficiente. Disponível: 5, Solicitado: 10"
}

# 4. Verificar que estoque NÃO foi debitado
GET /produtos/{id}
→ Resposta: { saldo: 5 }  ✓ Permanece intacto!

"Perfeito! A compensação funcionou automaticamente."
```

### Visualizações: RabbitMQ e Logs (2 min)
```
[Abrir http://localhost:15672 - RabbitMQ Management]

Login: admin / admin123

[Mostrar Exchanges]
"Aqui estão os exchanges topic configurados:
- faturamento-eventos
- estoque-eventos"

[Mostrar Queues]
"E as filas durables:
- estoque-eventos (2 mensagens processadas)
- faturamento-eventos (2 mensagens processadas)"

[Mostrar Bindings]
"Bindings conectam exchanges às queues com routing keys específicas."

[Terminal - Logs em tempo real]
$ docker logs -f servico-faturamento-1

[Mostrar logs]
✓ Evento publicado: Faturamento.ImpressaoSolicitada
✓ Processando: Estoque.Reservado
✓ Nota fechada com sucesso

$ docker logs -f servico-estoque-1

[Mostrar logs]
✓ Reserva criada para produto X
✓ Evento publicado: Estoque.Reservado

"Veja como os logs mostram o fluxo completo da Saga!"
```

---

## 🔍 PARTE 4: BANCO DE DADOS E VALIDAÇÕES (3 min)

### Query 1: Eventos Outbox
```
[Abrir DBeaver ou psql]

$ docker exec -it postgres-faturamento-1 psql -U admin -d faturamento

-- Ver eventos publicados
SELECT id, tipo_evento, 
       data_ocorrencia, 
       data_publicacao,
       payload::text
FROM eventos_outbox
ORDER BY data_ocorrencia DESC
LIMIT 5;

[Mostrar resultado]
ID | tipo_evento                        | data_publicacao
---+-----------------------------------+------------------
20 | Faturamento.ImpressaoSolicitada   | 2025-10-02 10:28:04
...

"Veja que todos os eventos foram publicados com sucesso!"
```

### Query 2: Mensagens Processadas (Idempotência)
```
$ docker exec -it postgres-estoque-1 psql -U admin -d estoque

-- Ver mensagens processadas
SELECT id_mensagem, 
       data_processada
FROM mensagens_processadas
ORDER BY data_processada DESC;

[Mostrar resultado]
id_mensagem                  | data_processada
-----------------------------+------------------
4                            | 2025-10-02 10:28:05
...

"Cada mensagem processada é registrada para evitar duplicação!"
```

### Query 3: Reservas de Estoque
```
-- Ver reservas criadas
SELECT r.id,
       r.nota_id,
       r.quantidade,
       r.status,
       p.nome as produto_nome,
       p.saldo
FROM reservas_estoque r
JOIN produtos p ON r.produto_id = p.id
ORDER BY r.data_criacao DESC;

[Mostrar resultado]
nota_id | quantidade | status    | produto_nome | saldo
--------+------------+-----------+--------------+------
...     | 30         | RESERVADO | Produto Demo | 70

"Reserva criada e saldo atualizado corretamente!"
```

---

## 🎓 PARTE 5: CONCLUSÃO E DESTAQUES (2 min)

### Resumo dos Padrões Implementados
```
"Recapitulando o que foi demonstrado:

✅ SAGA PATTERN coreografada
   - Comunicação via eventos assíncronos
   - Compensação automática em falhas

✅ TRANSACTIONAL OUTBOX
   - Atomicidade entre estado e eventos
   - Worker assíncrono para publicação

✅ IDEMPOTÊNCIA completa
   - HTTP: Idempotency-Key
   - Mensageria: mensagens_processadas

✅ CONCORRÊNCIA controlada
   - Otimista: xmin do PostgreSQL
   - Pessimista: SELECT FOR UPDATE

✅ RESILIÊNCIA
   - Retry automático
   - Timeouts configurados
   - ACK/NACK manual"
```

### Métricas e Resultados
```
"Resultados alcançados:

⏱️ Latência da Saga: ~2 segundos
✅ Taxa de sucesso: 100% nos testes
🔄 Rollback implícito: funciona perfeitamente
📊 Observabilidade: logs estruturados + RabbitMQ Management

O sistema está pronto para produção!"
```

### Próximos Passos
```
"Melhorias futuras que podem ser implementadas:

1. Dashboard de monitoramento (Grafana)
2. Dead Letter Queue para mensagens falhadas
3. Event Sourcing completo
4. CQRS com projeções em cache
5. Service Mesh (Istio) para observabilidade avançada"
```

### Agradecimento (30s)
```
"Obrigado por assistir! 

Código completo disponível no GitHub: [LINK]
Documentação técnica: README.md e ARQUITETURA.md

Qualquer dúvida, estou à disposição!

Até a próxima! 🚀"
```

---

## 📋 CHECKLIST PRÉ-GRAVAÇÃO

### Ambiente
- [ ] Docker Desktop rodando
- [ ] Todos os containers iniciados (`docker-compose up -d`)
- [ ] RabbitMQ Management acessível (http://localhost:15672)
- [ ] Postman configurado com requests prontas
- [ ] DBeaver conectado aos bancos PostgreSQL

### Ferramentas de Gravação
- [ ] OBS Studio ou similar configurado
- [ ] Microfone testado
- [ ] Resolução 1920x1080 (Full HD)
- [ ] Frame rate 30 FPS mínimo
- [ ] Webcam (opcional, canto inferior direito)

### Arquivos Abertos
- [ ] ARQUITETURA.md (VS Code)
- [ ] notas.go (endpoint de impressão)
- [ ] outbox.go (publicador)
- [ ] ConsumidorEventos.cs (consumidor)
- [ ] ContextoBancoDados.cs (configuração xmin)
- [ ] Terminal com docker-compose logs

### Postman/cURL
- [ ] Collection "Demo Sistema NFe" criada
- [ ] Variáveis de ambiente configuradas
- [ ] Requests na ordem:
  1. POST /produtos
  2. POST /notas
  3. POST /notas/{id}/itens
  4. POST /notas/{id}/imprimir
  5. GET /solicitacoes-impressao/{id}
  6. GET /produtos/{id}

### Ensaio
- [ ] Fazer 1 dry-run completo
- [ ] Verificar timing (não ultrapassar 20 min)
- [ ] Testar transições entre telas
- [ ] Validar áudio

---

## 🎥 DICAS DE GRAVAÇÃO

1. **Fale com clareza e pausadamente**
   - Evite jargões desnecessários
   - Explique siglas na primeira menção

2. **Mostre o código de forma limpa**
   - Zoom adequado (fonte >= 16pt)
   - Destaque linhas importantes com cursor

3. **Use anotações visuais**
   - Círculos vermelhos no OBS
   - Setas para indicar fluxo

4. **Mantenha energia**
   - Tom de voz variado
   - Entusiasmo ao mostrar sucessos

5. **Edição pós-gravação**
   - Cortar silêncios longos
   - Adicionar legendas nos pontos-chave
   - Intro/outro com logo/contato

---

## 📤 PÓS-PRODUÇÃO

### Upload
- [ ] YouTube (público ou não-listado)
- [ ] Google Drive (compartilhável)
- [ ] Adicionar timestamps na descrição

### Compartilhamento
- [ ] Link no README.md do GitHub
- [ ] Enviar para recrutador com:
  - Link do vídeo
  - Link do repositório
  - Resumo executivo (1 parágrafo)

**BOA SORTE! 🍀**
