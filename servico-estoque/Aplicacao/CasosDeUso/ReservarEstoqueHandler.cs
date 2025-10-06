using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Logging;
using ServicoEstoque.Dominio.Entidades;
using ServicoEstoque.Infraestrutura.Persistencia;
using System.Text.Json;

namespace ServicoEstoque.Aplicacao.CasosDeUso;

public sealed class ReservarEstoqueHandler
{
    private readonly ContextoBancoDados _ctx;
    private readonly ILogger<ReservarEstoqueHandler> _logger;

    public ReservarEstoqueHandler(ContextoBancoDados ctx, ILogger<ReservarEstoqueHandler> logger)
    {
        _ctx = ctx;
        _logger = logger;
    }

    public async Task<Resultado<ReservaEstoque>> Executar(
        ReservarEstoqueCommand cmd,
        bool simularFalha = false,
        CancellationToken ct = default)
    {
        await using var tx = await _ctx.Database.BeginTransactionAsync(ct);

        try
        {
            var produto = await _ctx.Produtos.FindAsync(new object[] { cmd.ProdutoId }, ct);
            if (produto is null)
                return Resultado<ReservaEstoque>.Falha("Produto não encontrado");

            _logger.LogInformation("[ReservarEstoque] Iniciando débito de estoque: Produto={ProdutoId}, Quantidade={Quantidade}", cmd.ProdutoId, cmd.Quantidade);
            var resultDebito = produto.DebitarEstoque(cmd.Quantidade);
            if (resultDebito.Falhou)
            {
                _logger.LogWarning("[ReservarEstoque] Débito rejeitado para Produto={ProdutoId}: {Motivo}", cmd.ProdutoId, resultDebito.Mensagem);

                // publica evento de rejeição
                _logger.LogInformation("[ReservarEstoque] Publicando evento de rejeição para Nota={NotaId}", cmd.NotaId);
                var eventoRejeicao = new EventoOutbox
                {
                    TipoEvento = "Estoque.ReservaRejeitada",
                    IdAgregado = cmd.NotaId,
                    Payload = JsonSerializer.Serialize(new
                    {
                        notaId = cmd.NotaId,
                        motivo = resultDebito.Mensagem
                    }),
                    DataOcorrencia = DateTime.UtcNow
                };
                _ctx.EventosOutbox.Add(eventoRejeicao);
                _logger.LogInformation("[ReservarEstoque] Salvando evento de rejeição antes do commit");
                await _ctx.SaveChangesAsync(ct);
                await tx.CommitAsync(ct);

                return Resultado<ReservaEstoque>.Falha(resultDebito.Mensagem!);
            }
            _logger.LogInformation("[ReservarEstoque] Débito aplicado com sucesso, saldo atual do produto: {Saldo}", produto.Saldo);

            var reserva = new ReservaEstoque
            {
                Id = Guid.NewGuid(),
                NotaId = cmd.NotaId,
                ProdutoId = cmd.ProdutoId,
                Quantidade = cmd.Quantidade,
                Status = "RESERVADO",
                DataCriacao = DateTime.UtcNow
            };
            _ctx.ReservasEstoque.Add(reserva);
            _logger.LogInformation("[ReservarEstoque] Reserva registrada: {ReservaId}", reserva.Id);

            var evento = new EventoOutbox
            {
                TipoEvento = "Estoque.Reservado",
                IdAgregado = cmd.NotaId,
                Payload = JsonSerializer.Serialize(new
                {
                    notaId = cmd.NotaId,
                    produtoId = cmd.ProdutoId,
                    quantidade = cmd.Quantidade
                }),
                DataOcorrencia = DateTime.UtcNow
            };
            _ctx.EventosOutbox.Add(evento);
            _logger.LogInformation("[ReservarEstoque] Evento de sucesso preparado para Nota={NotaId}", cmd.NotaId);

            // simula falha ANTES do commit
            if (simularFalha)
            {
                _logger.LogWarning("[ReservarEstoque] X-Demo-Fail detectado - lançando exceção antes de SaveChanges");
                throw new InvalidOperationException("Falha simulada");
            }

            _logger.LogInformation("[ReservarEstoque] Persistindo alterações...");
            await _ctx.SaveChangesAsync(ct);
            await tx.CommitAsync(ct);

            _logger.LogInformation("[ReservarEstoque] Reserva criada com sucesso: {ReservaId}", reserva.Id);
            return Resultado<ReservaEstoque>.Sucesso(reserva);
        }
        catch (DbUpdateConcurrencyException ex)
        {
            await tx.RollbackAsync(ct);
            _logger.LogWarning(ex, "[ReservarEstoque] Conflito de concorrência ao reservar estoque");

            // limpa rastreamento para evitar persistir estado inconsistente
            _ctx.ChangeTracker.Clear();

            // NOVA transação para publicar rejeição (contexto antigo foi abortado)
            await using var novatx = await _ctx.Database.BeginTransactionAsync(ct);
            try
            {
                var evt = new EventoOutbox
                {
                    TipoEvento = "Estoque.ReservaRejeitada",
                    IdAgregado = cmd.NotaId,
                    Payload = JsonSerializer.Serialize(new
                    {
                        notaId = cmd.NotaId,
                        motivo = "Conflito de concorrência"
                    }),
                    DataOcorrencia = DateTime.UtcNow
                };
                _ctx.EventosOutbox.Add(evt);
                await _ctx.SaveChangesAsync(ct);
                await novatx.CommitAsync(ct);
            }
            catch (Exception saveEx)
            {
                _logger.LogError(saveEx, "[ReservarEstoque] Falha ao publicar evento de rejeição após conflito");
                await novatx.RollbackAsync(ct);
            }

            return Resultado<ReservaEstoque>.Falha("Produto modificado. Tente novamente.");
        }
        catch (Exception ex)
        {
            await tx.RollbackAsync(ct);
            _logger.LogError(ex, "[ReservarEstoque] Erro ao processar reserva para NotaId={NotaId}", cmd.NotaId);

            // limpa rastreamento para evitar persistir estado inconsistente
            _ctx.ChangeTracker.Clear();

            // Publica evento de rejeição em NOVA transação
            await using var novatx = await _ctx.Database.BeginTransactionAsync(ct);
            try
            {
                var evt = new EventoOutbox
                {
                    TipoEvento = "Estoque.ReservaRejeitada",
                    IdAgregado = cmd.NotaId,
                    Payload = JsonSerializer.Serialize(new
                    {
                        notaId = cmd.NotaId,
                        motivo = ex.Message
                    }),
                    DataOcorrencia = DateTime.UtcNow
                };
                _ctx.EventosOutbox.Add(evt);
                await _ctx.SaveChangesAsync(ct);
                await novatx.CommitAsync(ct);
            }
            catch (Exception saveEx)
            {
                _logger.LogError(saveEx, "[ReservarEstoque] Falha ao publicar evento de rejeição após erro");
                await novatx.RollbackAsync(ct);
            }

            return Resultado<ReservaEstoque>.Falha($"Erro ao processar reserva: {ex.Message}");
        }
    }
}