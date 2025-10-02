package manipulador

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"servico-faturamento/internal/dominio"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Handlers struct {
	DB *gorm.DB
}

// POST /api/v1/notas
func (h *Handlers) CriarNota(c *gin.Context) {
	var req struct {
		Numero string `json:"numero" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": err.Error()})
		return
	}

	nota := dominio.NotaFiscal{
		Numero: req.Numero,
		Status: dominio.StatusNotaAberta,
	}

	if err := h.DB.Create(&nota).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"erro": "Falha ao criar nota"})
		return
	}

	c.JSON(http.StatusCreated, nota)
}

// GET /api/v1/notas
func (h *Handlers) ListarNotas(c *gin.Context) {
	var notas []dominio.NotaFiscal

	query := h.DB.Preload("Itens")
	
	// filtro opcional por status
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Find(&notas).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"erro": "Falha ao listar notas"})
		return
	}

	c.JSON(http.StatusOK, notas)
}

// GET /api/v1/notas/:id
func (h *Handlers) BuscarNota(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "ID inválido"})
		return
	}

	var nota dominio.NotaFiscal
	if err := h.DB.Preload("Itens").First(&nota, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"erro": "Nota não encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"erro": "Falha ao buscar nota"})
		return
	}

	c.JSON(http.StatusOK, nota)
}

// POST /api/v1/notas/:id/itens
func (h *Handlers) AdicionarItem(c *gin.Context) {
	notaID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "ID inválido"})
		return
	}

	var req struct {
		ProdutoID     string  `json:"produtoId" binding:"required"`
		Quantidade    int     `json:"quantidade" binding:"required,min=1"`
		PrecoUnitario float64 `json:"precoUnitario" binding:"required,min=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": err.Error()})
		return
	}

	prodID, err := uuid.Parse(req.ProdutoID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "ProdutoID inválido"})
		return
	}

	// verificar se nota existe e está aberta
	var nota dominio.NotaFiscal
	if err := h.DB.First(&nota, "id = ?", notaID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"erro": "Nota não encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"erro": "Falha ao buscar nota"})
		return
	}

	if nota.Status != dominio.StatusNotaAberta {
		c.JSON(http.StatusConflict, gin.H{"erro": "Nota não está aberta"})
		return
	}

	item := dominio.ItemNota{
		NotaID:        notaID,
		ProdutoID:     prodID,
		Quantidade:    req.Quantidade,
		PrecoUnitario: req.PrecoUnitario,
	}

	if err := h.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"erro": "Falha ao adicionar item"})
		return
	}

	c.JSON(http.StatusCreated, item)
}

// POST /api/v1/notas/:id/imprimir
func (h *Handlers) ImprimirNota(c *gin.Context) {
	notaID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "ID inválido"})
		return
	}

	chaveIdem := c.GetHeader("Idempotency-Key")
	if chaveIdem == "" {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "Header Idempotency-Key obrigatório"})
		return
	}

	// verificar se já existe solicitação com essa chave (idempotência)
	var solExistente dominio.SolicitacaoImpressao
	if err := h.DB.Where("chave_idempotencia = ?", chaveIdem).First(&solExistente).Error; err == nil {
		// já existe, retorna mesma resposta
		c.JSON(http.StatusOK, solExistente)
		return
	}

	// verificar se nota existe
	var nota dominio.NotaFiscal
	if err := h.DB.First(&nota, "id = ?", notaID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"erro": "Nota não encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"erro": "Falha ao buscar nota"})
		return
	}

	if nota.Status != dominio.StatusNotaAberta {
		c.JSON(http.StatusConflict, gin.H{"erro": "Nota não está aberta"})
		return
	}

	// buscar itens da nota pra enviar no evento
	var itens []dominio.ItemNota
	if err := h.DB.Where("nota_id = ?", notaID).Find(&itens).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"erro": "Falha ao buscar itens"})
		return
	}

	if len(itens) == 0 {
		c.JSON(http.StatusConflict, gin.H{"erro": "Nota sem itens não pode ser impressa"})
		return
	}

	// criar solicitação + publicar evento no outbox (transação garante atomicidade)
	err = h.DB.Transaction(func(tx *gorm.DB) error {
		sol := dominio.SolicitacaoImpressao{
			NotaID:            notaID,
			Status:            "PENDENTE",
			ChaveIdempotencia: chaveIdem,
		}

		if err := tx.Create(&sol).Error; err != nil {
			return err
		}

		// montar payload JSON com lista de itens pra reservar
		type itemEvento struct {
			ProdutoID  string `json:"produtoId"`
			Quantidade int    `json:"quantidade"`
		}

		type payloadEvento struct {
			NotaID string        `json:"notaId"`
			Itens  []itemEvento  `json:"itens"`
		}

		var itensEvento []itemEvento
		for _, item := range itens {
			itensEvento = append(itensEvento, itemEvento{
				ProdutoID:  item.ProdutoID.String(),
				Quantidade: item.Quantidade,
			})
		}

		payload := payloadEvento{
			NotaID: notaID.String(),
			Itens:  itensEvento,
		}

		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("falha ao serializar payload: %w", err)
		}

		// inserir evento no outbox (será publicado pelo background worker)
		eventoOutbox := dominio.EventoOutbox{
			TipoEvento:     "Faturamento.ImpressaoSolicitada",
			IdAgregado:     notaID,
			Payload:        string(payloadJSON),
			DataOcorrencia: time.Now(),
		}

		if err := tx.Create(&eventoOutbox).Error; err != nil {
			return fmt.Errorf("falha ao criar evento outbox: %w", err)
		}

		log.Printf("✓ Evento criado no outbox: %s para nota %s", eventoOutbox.TipoEvento, notaID)

		// sucesso: commit da transação
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"erro": fmt.Sprintf("Falha ao processar: %v", err)})
		return
	}

	// buscar a solicitação criada pra retornar
	var solCriada dominio.SolicitacaoImpressao
	if err := h.DB.Where("chave_idempotencia = ?", chaveIdem).First(&solCriada).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"erro": "Falha ao buscar solicitação"})
		return
	}

	c.JSON(http.StatusCreated, solCriada)
}

// GET /api/v1/solicitacoes-impressao/:id
func (h *Handlers) ConsultarStatusImpressao(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "ID inválido"})
		return
	}

	var sol dominio.SolicitacaoImpressao
	if err := h.DB.First(&sol, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"erro": "Solicitação não encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"erro": "Falha ao buscar solicitação"})
		return
	}

	c.JSON(http.StatusOK, sol)
}

// FecharNota é chamado pelo consumidor quando recebe "Estoque.Reservado"
func (h *Handlers) FecharNota(notaID uuid.UUID) error {
	return h.DB.Transaction(func(tx *gorm.DB) error {
		// busca com lock pessimista
		var nota dominio.NotaFiscal
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&nota, "id = ?", notaID).Error; err != nil {
			return err
		}

		if err := nota.Fechar(); err != nil {
			return err
		}

		if err := tx.Save(&nota).Error; err != nil {
			return err
		}

		// atualiza solicitação pra CONCLUIDA
		agora := time.Now()
		if err := tx.Model(&dominio.SolicitacaoImpressao{}).
			Where("nota_id = ? AND status = ?", notaID, "PENDENTE").
			Updates(map[string]interface{}{
				"status":          "CONCLUIDA",
				"data_conclusao": agora,
			}).Error; err != nil {
			return err
		}

		return nil
	})
}

// MarcarFalha é chamado quando recebe "Estoque.ReservaRejeitada"
func (h *Handlers) MarcarFalha(notaID uuid.UUID, motivo string) error {
	return h.DB.Model(&dominio.SolicitacaoImpressao{}).
		Where("nota_id = ? AND status = ?", notaID, "PENDENTE").
		Updates(map[string]interface{}{
			"status":         "FALHOU",
			"mensagem_erro": motivo,
		}).Error
}
