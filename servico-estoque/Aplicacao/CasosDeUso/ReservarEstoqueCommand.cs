namespace ServicoEstoque.Aplicacao.CasosDeUso;

public record ReservarEstoqueCommand(
    Guid NotaId,
    Guid ProdutoId,
    int Quantidade
);