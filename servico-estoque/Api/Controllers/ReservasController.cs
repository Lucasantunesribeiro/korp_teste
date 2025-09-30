using Microsoft.AspNetCore.Mvc;
using ServicoEstoque.Aplicacao.CasosDeUso;
using ServicoEstoque.Aplicacao.DTOs;

namespace ServicoEstoque.Api.Controllers;

[ApiController]
[Route("api/v1/reservas")]
public class ReservasController : ControllerBase
{
    private readonly ReservarEstoqueHandler _handler;
    private readonly ILogger<ReservasController> _logger;

    public ReservasController(ReservarEstoqueHandler handler, ILogger<ReservasController> logger)
    {
        _handler = handler;
        _logger = logger;
    }

    [HttpPost]
    public async Task<ActionResult> Reservar([FromBody] ReservarEstoqueRequest request)
    {
        // suporte pra testes: simular falha via header
        var demoFail = Request.Headers["X-Demo-Fail"].FirstOrDefault();
        bool simularFalha = demoFail == "true";

        if (simularFalha)
            _logger.LogWarning("Simulação de falha ativada via header X-Demo-Fail");

        var comando = new ReservarEstoqueCommand(
            request.NotaId,
            request.ProdutoId,
            request.Quantidade
        );

        var resultado = await _handler.Executar(comando, simularFalha);

        if (resultado.EhSucesso)
        {
            _logger.LogInformation(
                "Reserva OK: NotaId={NotaId}, ProdutoId={ProdutoId}, Qtd={Qtd}",
                request.NotaId, request.ProdutoId, request.Quantidade
            );

            return Ok(new
            {
                sucesso = true,
                reservaId = resultado.Dados!.Id,
                mensagem = "Reserva criada com sucesso"
            });
        }

        _logger.LogWarning("Falha ao reservar: {Erro}", resultado.Mensagem);

        return BadRequest(new
        {
            sucesso = false,
            mensagem = resultado.Mensagem
        });
    }
}