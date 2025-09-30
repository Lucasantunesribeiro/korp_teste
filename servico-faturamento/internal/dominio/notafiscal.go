package dominio

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotaFiscal struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Numero      string    `gorm:"unique;not null" json:"numero"`
	Status      string    `gorm:"not null" json:"status"` // ABERTA, FECHADA
	DataCriacao time.Time `gorm:"not null" json:"dataCriacao"`
	DataFechada *time.Time `json:"dataFechada,omitempty"`
	Itens       []ItemNota `gorm:"foreignKey:NotaID" json:"itens,omitempty"`
}

type ItemNota struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	NotaID        uuid.UUID  `gorm:"type:uuid;not null" json:"notaId"`
	ProdutoID     uuid.UUID  `gorm:"type:uuid;not null" json:"produtoId"`
	Quantidade    int        `gorm:"not null" json:"quantidade"`
	PrecoUnitario float64    `gorm:"type:decimal(10,2);not null" json:"precoUnitario"`
}

func (n *NotaFiscal) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.DataCriacao.IsZero() {
		n.DataCriacao = time.Now()
	}
	if n.Status == "" {
		n.Status = "ABERTA"
	}
	return nil
}

func (i *ItemNota) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

func (n *NotaFiscal) Fechar() error {
	if n.Status != "ABERTA" {
		return errors.New("nota não está aberta")
	}
	n.Status = "FECHADA"
	agora := time.Now()
	n.DataFechada = &agora
	return nil
}

func (n *NotaFiscal) TableName() string {
	return "notas_fiscais"
}

func (i *ItemNota) TableName() string {
	return "itens_nota"
}
