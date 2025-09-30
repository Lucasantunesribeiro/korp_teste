using Microsoft.EntityFrameworkCore;
using ServicoEstoque.Dominio.Entidades;

namespace ServicoEstoque.Infraestrutura.Persistencia;

public class ContextoBancoDados : DbContext
{
    public ContextoBancoDados(DbContextOptions<ContextoBancoDados> options) : base(options) { }

    public DbSet<Produto> Produtos => Set<Produto>();
    public DbSet<ReservaEstoque> ReservasEstoque => Set<ReservaEstoque>();
    public DbSet<EventoOutbox> EventosOutbox => Set<EventoOutbox>();

    protected override void OnModelCreating(ModelBuilder builder)
    {
        builder.Entity<Produto>(p =>
        {
            p.ToTable("produtos");
            p.HasKey(x => x.Id);

            p.Property(x => x.Id).HasColumnName("id");
            p.Property(x => x.Sku).HasColumnName("sku").HasMaxLength(50).IsRequired();
            p.Property(x => x.Nome).HasColumnName("nome").HasMaxLength(200).IsRequired();
            p.Property(x => x.Saldo).HasColumnName("saldo").IsRequired();
            p.Property(x => x.Ativo).HasColumnName("ativo").IsRequired();
            p.Property(x => x.DataCriacao).HasColumnName("data_criacao").IsRequired();

            // usa xmin pq concorrência otimista é mais eficiente aqui
            p.Property(x => x.Versao)
                .HasColumnName("xmin")
                .HasColumnType("xid")
                .IsRowVersion()
                .ValueGeneratedOnAddOrUpdate();

            p.HasIndex(x => x.Sku).IsUnique();
        });

        builder.Entity<ReservaEstoque>(r =>
        {
            r.ToTable("reservas_estoque");
            r.HasKey(x => x.Id);

            r.Property(x => x.Id).HasColumnName("id");
            r.Property(x => x.NotaId).HasColumnName("nota_id").IsRequired();
            r.Property(x => x.ProdutoId).HasColumnName("produto_id").IsRequired();
            r.Property(x => x.Quantidade).HasColumnName("quantidade").IsRequired();
            r.Property(x => x.Status).HasColumnName("status").HasMaxLength(20).IsRequired();
            r.Property(x => x.DataCriacao).HasColumnName("data_criacao").IsRequired();

            r.HasOne(x => x.Produto)
                .WithMany()
                .HasForeignKey(x => x.ProdutoId)
                .OnDelete(DeleteBehavior.Restrict);
        });

        builder.Entity<EventoOutbox>(e =>
        {
            e.ToTable("eventos_outbox");
            e.HasKey(x => x.Id);

            e.Property(x => x.Id).HasColumnName("id");
            e.Property(x => x.TipoEvento).HasColumnName("tipo_evento").HasMaxLength(100).IsRequired();
            e.Property(x => x.IdAgregado).HasColumnName("id_agregado").IsRequired();
            e.Property(x => x.Payload).HasColumnName("payload").HasColumnType("jsonb").IsRequired();
            e.Property(x => x.DataOcorrencia).HasColumnName("data_ocorrencia").IsRequired();
            e.Property(x => x.DataPublicacao).HasColumnName("data_publicacao");
            e.Property(x => x.TentativasEnvio).HasColumnName("tentativas_envio").HasDefaultValue(0);

            e.HasIndex(x => x.DataPublicacao)
                .HasFilter("data_publicacao IS NULL")
                .HasDatabaseName("idx_outbox_pendentes");
        });
    }
}