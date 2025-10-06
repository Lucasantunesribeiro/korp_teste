# ROTEIRO DO VÃDEO

> **Projeto:** Viasoft Korp ERP â€“ EstÃ¡gio C#, Golang e Angular  
> **Escopo:** Fluxo completo de emissÃ£o de notas fiscais com microserviÃ§os (estoque + faturamento), front-end Angular e demonstraÃ§Ã£o prÃ¡tica de ACID, idempotÃªncia, tolerÃ¢ncia a falhas e concorrÃªncia.  
> **Entrega:** VÃ­deo de atÃ© 20 minutos (YouTube/Drive) + link do GitHub com o projeto.

## Como usar este roteiro

- FaÃ§a um ensaio completo seguindo cada cena antes de gravar oficialmente.
- â€œPara vocÃªâ€ = checklist de preparaÃ§Ã£o (ambiente, comandos, telas, scripts).
- â€œExplicaÃ§Ã£oâ€ = texto base para narrar. Adapte Ã  sua voz, mantenha tom claro e empolgante.
- Fale em ritmo confortÃ¡vel (~140 palavras/min), respirando e mantendo contato com a cÃ¢mera.
- Mostre sempre a tela correta (cÃ³digo, terminal, navegador ou RabbitMQ) enquanto comenta.

---

## ðŸ” PreparaÃ§Ã£o antes da gravaÃ§Ã£o

1. Assista ao briefing oficial (https://youtu.be/Iu2pKbFsdzA) e anote os requisitos reforÃ§ados.
2. Feche aplicaÃ§Ãµes ruidosas, silencie notificaÃ§Ãµes e teste Ã¡udio/webcam.
3. Limpe o terminal (`clear`/`cls`) e aumente a fonte do VS Code (130â€“140%).
4. Confirme que o repositÃ³rio local estÃ¡ atualizado (branch `main`).
5. Ajuste iluminaÃ§Ã£o e enquadramento da cÃ¢mera.

### Ambiente tÃ©cnico
- **Para vocÃª:**
  1. Derrube qualquer stack anterior:
     ```bash
     docker compose down -v
     docker system prune -af --volumes
     ```
  2. Suba toda a stack limpa:
     ```bash
     docker compose up -d --build
     sleep 60
     docker ps
     ```
  3. Valide a saÃºde dos serviÃ§os:
     ```bash
     curl http://localhost:5001/health
     curl http://localhost:5001/api/v1/produtos
     curl http://localhost:5002/health
     ```
  4. Inicie o front-end em outro terminal:
     ```bash
     cd web-app
     npm install          # apenas na primeira vez
     npm run start        # usa proxy.conf.json
     ```
  5. Abra no navegador `http://localhost:4200` (Produtos) e `http://localhost:15672` (RabbitMQ â€“ admin/admin123). Deixe tambÃ©m um terminal com logs:
     ```bash
     docker compose logs --tail 20 -f servico-estoque
     docker compose logs --tail 20 -f servico-faturamento
     ```

---

## â±ï¸ Planejamento do vÃ­deo (â‰¤ 20 min)

| Parte | TÃ­tulo | DuraÃ§Ã£o alvo |
|-------|--------|---------------|
| 1 | Contexto do desafio + vaga | 3 min |
| 2 | Arquitetura e requisitos tÃ©cnicos | 5 min |
| 3 | Tour pelo cÃ³digo (estoque, faturamento, Angular) | 5 min |
| 4 | DemonstraÃ§Ãµes ao vivo (feliz, falha, concorrÃªncia) | 6 min |
| 5 | Observabilidade + conclusÃ£o | 1 min |

---

## ðŸŽ¬ Parte 1 â€” Contexto e objetivo (3 min)

### Cena 1 â€” Cumprimento + vaga
- **Para vocÃª:** webcam ligada, slide simples com o logo Viasoft Korp + tÃ­tulo. Respire fundo antes de iniciar.
- **ExplicaÃ§Ã£o:**
  > "Oi pessoal, eu sou o Lucas e estou participando do processo de EstÃ¡gio em C#, Golang e Angular da Viasoft Korp. A vaga busca alguÃ©m para apoiar a criaÃ§Ã£o e testes de componentes, dar suporte a serviÃ§os REST e crescer junto com o time, misturando curiosidade tÃ©cnica com paixÃ£o pela indÃºstria."  
  > "A empresa estÃ¡ hÃ¡ mais de 20 anos ajudando indÃºstrias com tecnologia, e esse desafio Ã© a chance de mostrar como eu entrego valor de forma prÃ¡tica."  
  > "Recebi um escopo de 3 dias para construir uma soluÃ§Ã£o de emissÃ£o de notas fiscais com microserviÃ§os. Neste vÃ­deo vou explicar as escolhas arquiteturais, mostrar o cÃ³digo e provar que tudo funciona, inclusive quando algo dÃ¡ errado."  
  > "Antes de entrar no cÃ³digo, vou relembrar rapidamente o que o teste exige e como organizei o projeto."

### Cena 2 â€” Requisitos do teste
- **Para vocÃª:** exiba um slide/nota com os requisitos principais.
- **ExplicaÃ§Ã£o:**
  > "O desafio pede trÃªs funcionalidades principais: cadastrar produtos com controle de saldo, cadastrar notas fiscais com itens e permitir a impressÃ£o da nota. Na impressÃ£o preciso validar estoque, baixar o saldo, mudar o status da nota e dar feedback ao usuÃ¡rio."  
  > "Arquiteturalmente, Ã© obrigatÃ³rio dividir em pelo menos dois microserviÃ§os â€” um de estoque, outro de faturamento â€” garantindo transaÃ§Ãµes ACID. TambÃ©m preciso demonstrar uma falha recuperÃ¡vel e, como desafio extra, lidar com concorrÃªncia."  
  > "AlÃ©m disso, o processo seletivo envolve etapas de cadastro, testes lÃ³gico e comportamental, contato com RH e contrataÃ§Ã£o. A vaga pode ser remota ou presencial, horÃ¡rio das 8h30 Ã s 17h30, com benefÃ­cios como auxÃ­lio home office e TotalPass. EntÃ£o quero que este vÃ­deo comprove tanto o domÃ­nio tÃ©cnico quanto a postura colaborativa que o time busca."  
  > "Vou mostrar como estruturei tudo isso e como cada requisito aparece na prÃ¡tica."

---

## ðŸ—ï¸ Parte 2 â€” Arquitetura e conceitos (5 min)

### Cena 3 â€” Diagrama geral
- **Para vocÃª:** abra `ARQUITETURA.md` com zoom em 150%.
- **ExplicaÃ§Ã£o:**
  > "Aqui estÃ¡ a visÃ£o macro: dois microserviÃ§os (Go e C#) rodando em containers separados, RabbitMQ para eventos e uma SPA Angular via proxy."  
  > "O faturamento em Go orquestra a criaÃ§Ã£o da nota e publica eventos quando precisa imprimir. O estoque em C# responde a esses eventos, valida saldo, aplica concorrÃªncia otimista e devolve eventos de sucesso ou rejeiÃ§Ã£o."  
  > "Cada serviÃ§o tem seu banco Postgres (Database-per-Service). O Angular consome via proxy local (ng serve) ou via Nginx no container."  
  > "A comunicaÃ§Ã£o segue o Saga Pattern coreografado: quem muda estado publica evento, e quem recebe reage de forma autÃ´noma."

### Cena 4 â€” ACID + Outbox + IdempotÃªncia
- **Para vocÃª:** role em `ARQUITETURA.md` atÃ© os tÃ³picos relevantes.
- **ExplicaÃ§Ã£o:**
  > "Para garantir ACID, cada operaÃ§Ã£o crÃ­tica roda em transaÃ§Ãµes locais. No C#, uso EF Core com `BeginTransactionAsync` e sÃ³ dou `SaveChangesAsync` apÃ³s preparar os eventos. Em Go, o GORM usa `SELECT ... FOR UPDATE` para lock pessimista quando fecha a nota."  
  > "O Outbox Pattern garante atomicidade: toda mudanÃ§a de estado registra um evento em `eventos_outbox`. Um background service lÃª e publica no RabbitMQ antes de marcar como entregue."  
  > "IdempotÃªncia acontece em dois nÃ­veis. HTTP: via header `Idempotency-Key`, armazenando o ID de solicitaÃ§Ã£o. Mensageria: cada consumo registra a mensagem em `mensagens_processadas`, evitando reprocessamentos."  
  > "Esses trÃªs recursos juntos entregam consistÃªncia eventual mesmo com falhas de rede ou duplicidade de mensagens."

---

## ðŸ’» Parte 3 â€” Tour pelo cÃ³digo (5 min)

### Cena 5 â€” ServiÃ§o de Estoque (C#)
- **Para vocÃª:** abra `servico-estoque/Api/Program.cs`, `Aplicacao/CasosDeUso/ReservarEstoqueHandler.cs`, `Infraestrutura/Mensageria/ConsumidorEventos.cs`.
- **ExplicaÃ§Ã£o:**
  > "No estoque uso .NET 9. O `Program.cs` registra DbContext, handlers e background services; note o `app.MapGet("/health")` que evita 404 nas checagens."  
  > "`ReservarEstoqueHandler` mostra a transaÃ§Ã£o ACID: busca produto, debita usando concorrÃªncia otimista (`xmin`), cria reserva e registra eventos na outbox. Se o saldo Ã© insuficiente ou algo falha, publico `Estoque.ReservaRejeitada`."  
  > "`ConsumidorEventos` processa cada mensagem (prefetch=1), aplica idempotÃªncia e confirma via ACK manual. Isso garante que o estoque nunca fica negativo, mesmo se o RabbitMQ reenviar mensagens."

### Cena 6 â€” ServiÃ§o de Faturamento (Go)
- **Para vocÃª:** abra `servico-faturamento/cmd/api/main.go`, `internal/manipulador/notas.go`, `internal/consumidor/consumidor.go`.
- **ExplicaÃ§Ã£o:**
  > "No faturamento, `main.go` inicializa o banco, o publicador outbox e o consumidor. A rota `/health` facilita o monitoramento."  
  > "O handler `ImprimirNota` recebe `Idempotency-Key`, cria (ou reutiliza) uma solicitaÃ§Ã£o, grava no outbox dentro da mesma transaÃ§Ã£o e devolve o ID para polling."  
  > "O consumidor em Go escuta `Estoque.Reservado` e `Estoque.ReservaRejeitada`, aplica `SELECT ... FOR UPDATE` via GORM para fechar notas sem corridas e registra mensagens processadas para idempotÃªncia."

### Cena 7 â€” Front-end Angular
- **Para vocÃª:** abra `web-app/src/app/features/produtos/produtos-lista.component.ts`, `features/notas/nota-detalhes.component.ts`, arquivos em `core/services` e `proxy.conf.json`.
- **ExplicaÃ§Ã£o:**
  > "O Angular 17 usa Signals para gerenciar estado simples. Em `produtos-lista` dÃ¡ para ver o cadastro e o feedback visual de saldo."  
  > "`nota-detalhes` executa o fluxo completo: adiciona itens, gera `Idempotency-Key`,   > "`nota-detalhes` executa o fluxo completo: adiciona itens, gera `Idempotency-Key`, inicia polling atÃ© sair de `PENDENTE` e exibe cards para processando, sucesso ou falha."  
  > "Os serviÃ§os chamam `/api/estoque` e `/api/faturamento`. O `proxy.conf.json` garante que `ng serve` encaminhe para as portas 5001 e 5002 sem alterar cÃ³digo."

### Cena 8 â€” ConcorrÃªncia e integridade
- **Para vocÃª:** prepare slides rÃ¡pidos para â€œOptimistic Lock (C#)â€ e â€œPessimistic Lock (Go)â€. Mostre `ReservarEstoqueHandler.cs` (tratamento de `DbUpdateConcurrencyException`) e `consumidor.go` (uso de `clause.Locking`).
- **ExplicaÃ§Ã£o:**
  > "O desafio extra pedia concorrÃªncia. No estoque, uso o optimistic locking do Postgres (`xmin`); se dois usuÃ¡rios disputam, um commit falha e publico `Estoque.ReservaRejeitada`."  
  > "No faturamento, uso `SELECT ... FOR UPDATE` via GORM para fechar notas sem corridas. Uma mensagem por vez modifica a nota."  
  > "Combinando os dois, mantenho consistÃªncia eventual sem travar o sistema inteiro."

---

## ðŸš€ Parte 4 â€” DemonstraÃ§Ãµes (6 min)

### Cena 9 â€” Fluxo feliz (estoque suficiente)
- **Para vocÃª:** mantenha a aplicaÃ§Ã£o Angular em destaque (zoom 120%) e um terminal com logs por perto.
- **Passos:**
  1. Produtos â†’ `+ Novo Produto` â†’ `SKU: NTB-001`, `Nome: Notebook Dell`, `Saldo: 10`.
  2. Notas â†’ `+ Nova Nota` â†’ `NÃºmero: NFE-001`.
  3. Abrir a nota, adicionar item `Notebook Dell`, `Quantidade: 3`, `PreÃ§o: 2000,00`.
  4. Clicar `ðŸ–¨ï¸ Solicitar ImpressÃ£o` e mostrar o card azul ðŸ¡’ verde, com status mudando para `FECHADA`.
  5. Voltar a Produtos e mostrar saldo `7`.
- **ExplicaÃ§Ã£o:**
  > "Aqui vemos o fluxo feliz: o saldo Ã© validado, a nota Ã© impressa e o card verde sÃ³ aparece quando `Estoque.Reservado` volta e o faturamento fecha a nota."

### Cena 10 â€” Falha simulada (saldo insuficiente)
- **Para vocÃª:** permaneÃ§a no navegador; â€œNotebook Dellâ€ ficou com saldo 7 apÃ³s o cenÃ¡rio feliz.
- **Passos:**
  1. Notas â†’ `+ Nova Nota` â†’ `NÃºmero: NFE-002`, abrir detalhes.
  2. Adicionar item â€œNotebook Dellâ€ com `Quantidade: 9` (maior que o saldo restante).
  3. Clicar `ðŸ–¨ï¸ Solicitar ImpressÃ£o` â†’ aparece card vermelho â€œSaldo insuficienteâ€.
  4. Voltar a Produtos e mostrar que o saldo continua 7.
- **ExplicaÃ§Ã£o:**
  > "Reaproveitando o produto, demonstramos a falha: `Estoque.ReservaRejeitada` Ã© disparado, o faturamento marca a solicitaÃ§Ã£o como FALHOU e o saldo nÃ£o Ã© alterado."

### Cena 11 â€” Falha controlada (X-Demo-Fail)
- **Para vocÃª:** execute o script de rollback (usa `http://` completo e compara saldo antes/depois):
  ```powershell
  powershell -File ./scripts/test-rollback-final.ps1
  ```
- **Expectativa para a gravaÃ§Ã£o:**
  1. O script mostra o produto/nota escolhidos e o JSON do body.
  2. A chamada com `X-Demo-Fail=true` retorna **Status HTTP 400** (Falha simulada).
  3. ApÃ³s 3 segundos, o script mostra â€œSaldo ANTESâ€ = â€œSaldo DEPOISâ€ e escreve `ROLLBACK FUNCIONOU!`.
  4. Se quiser reforÃ§ar, abra `docker compose logs servico-estoque --tail 20 -f` e destaque `X-Demo-Fail detectado - lanÃ§ando exceÃ§Ã£o antes de SaveChanges`.
- **ExplicaÃ§Ã£o:**
  > "Rodando `test-rollback-final.ps1`, a API responde 400 com 'Falha simulada' e o script confirma que o saldo nÃ£o mudou (`DiferenÃ§a: 0`). Encerramos narrando 'ROLLBACK FUNCIONOU! O saldo nÃ£o mudou, provando que a transaÃ§Ã£o foi revertida', e podemos mostrar o log com `X-Demo-Fail detectado - lanÃ§ando exceÃ§Ã£o antes de SaveChanges` para reforÃ§ar."

### Cena 12 â€” ConcorrÃªncia (lock otimista + pessimismo)
- **Para vocÃª:** rode o script que simula duas reservas simultÃ¢neas:
  ```powershell
  powershell -File ./scripts/test-concurrency.ps1
  ```
- **Expectativa para a gravaÃ§Ã£o:**
  1. Uma resposta aparece como **SUCESSO 200**, a outra como **FALHA (400/409)** com JSON de erro.
  2. O saldo final impresso Ã© `2`, mostrando que apenas uma reserva foi efetivada.
  3. Mostre rapidamente `docker compose logs servico-estoque --tail 20 -f` para destacar `Conflito de concorrÃªncia ao reservar estoque`.
- **ExplicaÃ§Ã£o:**
  > "Duas requisiÃ§Ãµes simultÃ¢neas disputam o mesmo estoque. O lock otimista no C# garante que uma vence e a outra cai no `DbUpdateConcurrencyException`, gerando `Estoque.ReservaRejeitada`. O script mostra um sucesso, uma falha e saldo final 2 (de 5), provando que nÃ£o hÃ¡ dÃ©bito duplicado."

---

## ðŸ“Š Parte 5 â€” Observabilidade (2 min)

### Cena 13 â€” RabbitMQ + logs
- **Para vocÃª:** divida a tela â€“ esquerda RabbitMQ (`Exchanges`, `Queues`), direita terminal.
  1. Abra `http://localhost:15672`, faÃ§a login com `admin` / `admin123`.
  2. Na aba **Overview**, mostre que as filas estÃ£o vazias apÃ³s processar as mensagens (grÃ¡fico zerado).
  3. Entre em **Exchanges** e destaque `faturamento-eventos` e `estoque-eventos`.
  4. Volte em **Queues** e mostre que nÃ£o hÃ¡ mensagens pendentes.
  5. No terminal, rode os logs mostrando os Ãºltimos eventos:
     ```bash
     docker compose logs --tail 20 -f servico-faturamento
     docker compose logs --tail 20 -f servico-estoque
     ```
     (se quiser, aplique um `Select-String "Reservado|Conflito|Simulada"` para destacar as linhas relevantes). Se surgir evento antigo para nota que já não existe, o Go registra `Mensagem ... marcada como ignorada` e encerra sem novos retries.
- **ExplicaÃ§Ã£o:**
  > "Aqui estÃ£o os bastidores. Em `faturamento-eventos` vejo o envio dos pedidos; em `estoque-eventos`, as confirmaÃ§Ãµes ou rejeiÃ§Ãµes. No fluxo feliz, as filas ficam vazias â€” sinal de que as mensagens foram entregues."  
  > "Nos logs do Go aparecem linhas como `Nota fechada com sucesso` ou `Mensagem XYZ marcada como ignorada` (quando a nota jÃ¡ nÃ£o existe), e no .NET vejo `Reserva criada`, `Falha simulada` ou `Conflito de concorrÃªncia`. Se algo quebrar em produÃ§Ã£o, esse Ã© o primeiro lugar a investigar."

---

## ðŸŽ“ Parte 6 â€” ConclusÃ£o (1 min)

### Cena 14 â€” Fechamento
- **Para vocÃª:** volte Ã  webcam, slide final com tÃ³picos chave.
- **ExplicaÃ§Ã£o:**
  > "Para fechar: entreguei os requisitos com microserviÃ§os independentes, transaÃ§Ãµes ACID, compensaÃ§Ãµes via Saga e feedback completo. Demonstrei cenÃ¡rio feliz, falha e concorrÃªncia controlada."  
  > "Essas escolhas mostram como posso contribuir com o ERP, com os serviÃ§os REST e com as telas que o time atualiza no dia a dia, alinhado Ã  cultura de pessoas em primeiro lugar."  
  > "PrÃ³ximos passos: instrumentar mÃ©tricas com Prometheus + Grafana, adicionar testes de carga automatizados e preparar deploy em Kubernetes. Obrigado pela oportunidade â€” aguardo o feedback!"

---

## âœ… Checklist final

- [ ] Reassistir o briefing e revisar slides.
- [ ] Confirmar containers `healthy` e `docker ps` limpo.- [ ] Deixar comandos de demo prontos (estoque, faturamento, falha, concorrÃªncia).
- [ ] Ensaiar tempo total (< 20 min).
- [ ] Conferir iluminaÃ§Ã£o, Ã¡udio, webcam e foco antes do REC.
- [ ] ApÃ³s gravar: cortar silÃªncios, adicionar transiÃ§Ãµes suaves e exportar em 1080p 60fps.
- [ ] Subir o vÃ­deo (YouTube/Drive como â€œnÃ£o listadoâ€) e validar o link.

Boa gravaÃ§Ã£o! ðŸ’ª
