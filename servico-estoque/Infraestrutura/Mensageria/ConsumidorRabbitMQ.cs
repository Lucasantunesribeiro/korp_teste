using System.Text;
using System.Text.Json;
using Microsoft.EntityFrameworkCore;
using RabbitMQ.Client;
using RabbitMQ.Client.Events;
using ServicoEstoque.Aplicacao.CasosDeUso;
using ServicoEstoque.Aplicacao.DTOs;
using ServicoEstoque.Infraestrutura.Persistencia;

namespace ServicoEstoque.Infraestrutura.Mensageria;

/// <summary>
/// Consumidor de eventos do RabbitMQ para processar solicitações de reserva vindas do Faturamento
/// Implementa idempotência e processamento transacional
/// </summary>
public class ConsumidorRabbitMQ : BackgroundService
{
    private readonly IServiceProvider _serviceProvider;
    private readonly ILogger<ConsumidorRabbitMQ> _logger;
    private IConnection? _conexao;
    private IModel? _canal;

    public ConsumidorRabbitMQ(IServiceProvider serviceProvider, ILogger<ConsumidorRabbitMQ> logger)
    {
        _serviceProvider = serviceProvider;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        await InicializarConexao(stoppingToken);

        if (_canal is null)
        {
            _logger.LogError("Não foi possível estabelecer conexão com RabbitMQ após várias tentativas");
            return;
        }

        ConfigurarConsumidor();

        _logger.LogInformation("Consumidor RabbitMQ iniciado, aguardando eventos...");

        // manter serviço vivo enquanto app rodar
        await Task.Delay(Timeout.Infinite, stoppingToken);
    }

    private async Task InicializarConexao(CancellationToken ct)
    {
        var host = Environment.GetEnvironmentVariable("RabbitMQ__Host") ?? "rabbitmq";
        var usuario = Environment.GetEnvironmentVariable("RabbitMQ__Username") ?? "admin";
        var senha = Environment.GetEnvironmentVariable("RabbitMQ__Password") ?? "admin123";

        var factory = new ConnectionFactory
        {
            HostName = host,
            UserName = usuario,
            Password = senha,
            AutomaticRecoveryEnabled = true,
            NetworkRecoveryInterval = TimeSpan.FromSeconds(10)
        };

        // retry: rabbitmq demora pra subir no docker
        for (int tentativa = 1; tentativa <= 15; tentativa++)
        {
            try
            {
                _logger.LogInformation("Tentativa {Tentativa}/15 de conexão com RabbitMQ...", tentativa);
                _conexao = factory.CreateConnection();
                _canal = _conexao.CreateModel();

                _logger.LogInformation("✓ Conectado ao RabbitMQ com sucesso");
                return;
            }
            catch (Exception ex)
            {
                _logger.LogWarning("Falha na tentativa {Tentativa}: {Erro}", tentativa, ex.Message);

                if (tentativa < 15)
                    await Task.Delay(3000, ct);
            }
        }
    }

    private void ConfigurarConsumidor()
    {
        if (_canal is null) return;

        // declarar exchange (topic pattern pra rotear eventos)
        _canal.ExchangeDeclare(
            exchange: "faturamento-eventos",
            type: ExchangeType.Topic,
            durable: true
        );

        // criar fila exclusiva do estoque
        var nomeFila = _canal.QueueDeclare(
            queue: "estoque-eventos",
            durable: true,
            exclusive: false,
            autoDelete: false,
            arguments: null
        ).QueueName;

        // bind: receber eventos de solicitação de impressão (que precisam reservar estoque)
        _canal.QueueBind(
            queue: nomeFila,
            exchange: "faturamento-eventos",
            routingKey: "Faturamento.ImpressaoSolicitada"
        );

        // QoS: processar 1 mensagem por vez (evita concorrência interna)
        _canal.BasicQos(prefetchSize: 0, prefetchCount: 1, global: false);

        var consumidor = new EventingBasicConsumer(_canal);
        consumidor.Received += async (modelo, args) =>
        {
            try
            {
                await ProcessarMensagem(args);
                _canal.BasicAck(deliveryTag: args.DeliveryTag, multiple: false);
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Erro ao processar mensagem {MsgId}", args.BasicProperties.MessageId);

                // NACK com requeue=false: mensagem vai pro DLQ (dead letter queue)
                _canal.BasicNack(deliveryTag: args.DeliveryTag, multiple: false, requeue: false);
            }
        };

        _canal.BasicConsume(
            queue: nomeFila,
            autoAck: false, // ACK manual pra garantir processamento
            consumer: consumidor
        );
    }

    private async Task ProcessarMensagem(BasicDeliverEventArgs args)
    {
        using var escopo = _serviceProvider.CreateScope();
        var contexto = escopo.ServiceProvider.GetRequiredService<ContextoBancoDados>();
        var handler = escopo.ServiceProvider.GetRequiredService<ReservarEstoqueHandler>();

        // idempotência: usar MessageId único do RabbitMQ
        var idMensagem = args.BasicProperties.MessageId ?? $"delivery-{args.DeliveryTag}";

        // verificar se já processamos essa msg (evita duplicação em retry)
        var jaProcessada = await contexto.Database
            .SqlQuery<int>($"SELECT COUNT(*) FROM mensagens_processadas WHERE id_mensagem = {idMensagem}")
            .FirstOrDefaultAsync();

        if (jaProcessada > 0)
        {
            _logger.LogInformation("Mensagem {MsgId} já foi processada anteriormente, ignorando", idMensagem);
            return;
        }

        // deserializar payload JSON
        var corpo = Encoding.UTF8.GetString(args.Body.ToArray());
        var evento = JsonSerializer.Deserialize<EventoSolicitacaoImpressao>(corpo, new JsonSerializerOptions
        {
            PropertyNameCaseInsensitive = true
        });

        if (evento is null)
        {
            _logger.LogError("Falha ao deserializar evento: {Corpo}", corpo);
            return;
        }

        _logger.LogInformation(
            "Processando solicitação de reserva para nota {NotaId} com {QtdItens} itens",
            evento.NotaId, evento.Itens.Count
        );

        // processar cada item da nota (reservar estoque)
        foreach (var item in evento.Itens)
        {
            var comando = new ReservarEstoqueCommand(
                NotaId: evento.NotaId,
                ProdutoId: item.ProdutoId,
                Quantidade: item.Quantidade
            );

            // handler já tem transação, outbox e trata concorrência
            var resultado = await handler.Executar(comando, simularFalha: false);

            if (resultado.Falhou)
            {
                _logger.LogWarning(
                    "Reserva rejeitada para produto {ProdutoId}: {Motivo}",
                    item.ProdutoId, resultado.Mensagem
                );
                // evento de rejeição já foi publicado pelo handler
            }
            else
            {
                _logger.LogInformation(
                    "✓ Reserva criada para produto {ProdutoId}, quantidade {Qtd}",
                    item.ProdutoId, item.Quantidade
                );
            }
        }

        // marcar mensagem como processada (idempotência)
        await contexto.Database.ExecuteSqlRawAsync(
            "INSERT INTO mensagens_processadas (id_mensagem, data_processada) VALUES ({0}, {1})",
            idMensagem, DateTime.UtcNow
        );

        _logger.LogInformation("Solicitação {NotaId} processada com sucesso", evento.NotaId);
    }

    public override void Dispose()
    {
        _canal?.Close();
        _conexao?.Close();
        base.Dispose();
    }
}

// DTOs internos para deserializar eventos
internal record EventoSolicitacaoImpressao(
    Guid NotaId,
    List<ItemNotaEvento> Itens
);

internal record ItemNotaEvento(
    Guid ProdutoId,
    int Quantidade
);
