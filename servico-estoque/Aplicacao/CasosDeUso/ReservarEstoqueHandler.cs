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

            var resultDebito = produto.DebitarEstoque(cmd.Quantidade);
            if (resultDebito.Falhou)
            {
                // publica evento de rejeição
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
                await _ctx.SaveChangesAsync(ct);
                await tx.CommitAsync(ct);

                return Resultado<ReservaEstoque>.Falha(resultDebito.Mensagem!);
            }

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

            // simula falha ANTES do commit
            if (simularFalha)
            {
                _logger.LogWarning("Falha simulada antes do commit - vai fazer rollback");
                throw new InvalidOperationException("Falha simulada");
            }

            await _ctx.SaveChangesAsync(ct);
            await tx.CommitAsync(ct);

            _logger.LogInformation("Reserva criada com sucesso: {ReservaId}", reserva.Id);
            return Resultado<ReservaEstoque>.Sucesso(reserva);
        }
        catch (DbUpdateConcurrencyException ex)
        {
            await tx.RollbackAsync(ct);
            _logger.LogWarning(ex, "Conflito de concorrência ao reservar estoque");

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
                _logger.LogError(saveEx, "Falha ao publicar evento de rejeição após conflito");
                await novatx.RollbackAsync(ct);
            }

            return Resultado<ReservaEstoque>.Falha("Produto modificado. Tente novamente.");
        }
        catch (Exception ex)
        {
            await tx.RollbackAsync(ct);
            _logger.LogError(ex, "Erro ao processar reserva para NotaId={NotaId}", cmd.NotaId);

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
                _logger.LogError(saveEx, "Falha ao publicar evento de rejeição após erro");
                await novatx.RollbackAsync(ct);
            }

            return Resultado<ReservaEstoque>.Falha($"Erro ao processar reserva: {ex.Message}");
        }
    }
}