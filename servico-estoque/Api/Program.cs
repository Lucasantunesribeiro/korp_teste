using System.Text.Json.Serialization;
using Microsoft.EntityFrameworkCore;
using ServicoEstoque.Aplicacao.CasosDeUso;
using ServicoEstoque.Infraestrutura.Mensageria;
using ServicoEstoque.Infraestrutura.Persistencia;

var builder = WebApplication.CreateBuilder(args);

// config de serviços
builder.Services.AddControllers()
    .AddJsonOptions(opts =>
    {
        opts.JsonSerializerOptions.ReferenceHandler = ReferenceHandler.IgnoreCycles;
        opts.JsonSerializerOptions.DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull;
    });

builder.Services.AddEndpointsApiExplorer();
builder.Services.AddSwaggerGen();

// dbcontext com npgsql
var connStr = Environment.GetEnvironmentVariable("ConnectionStrings__DefaultConnection")
    ?? "Host=postgres-estoque;Database=estoque;Username=admin;Password=admin123";

builder.Services.AddDbContext<ContextoBancoDados>(opts =>
    opts.UseNpgsql(connStr, npgsql =>
    {
        npgsql.CommandTimeout(30);
    })
);

// handlers
builder.Services.AddScoped<ReservarEstoqueHandler>();

// background services: outbox publisher + rabbitmq consumer
builder.Services.AddHostedService<PublicadorOutbox>();
builder.Services.AddHostedService<ConsumidorEventos>();

// cors pra angular
builder.Services.AddCors(opts =>
{
    opts.AddDefaultPolicy(policy =>
    {
        policy.WithOrigins("http://localhost:4200")
            .AllowAnyHeader()
            .AllowAnyMethod();
    });
});

var app = builder.Build();

// middleware pipeline
if (app.Environment.IsDevelopment())
{
    app.UseSwagger();
    app.UseSwaggerUI();
}

app.UseCors();
app.UseAuthorization();
app.MapControllers();

app.Run();