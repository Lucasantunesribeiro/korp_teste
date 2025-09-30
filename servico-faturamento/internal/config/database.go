package config

import (
	"fmt"
	"log"
	"os"

	"servico-faturamento/internal/dominio"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InicializarDB() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://admin:admin123@postgres-faturamento:5432/faturamento?sslmode=disable"
	}

	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	db, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar DB: %w", err)
	}

	log.Println("Conexão com PostgreSQL estabelecida")

	// AutoMigrate das tabelas
	err = db.AutoMigrate(
		&dominio.NotaFiscal{},
		&dominio.ItemNota{},
		&dominio.SolicitacaoImpressao{},
		&dominio.EventoOutbox{},
		&dominio.MensagemProcessada{},
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao executar migrations: %w", err)
	}

	log.Println("Migrations aplicadas com sucesso")

	return db, nil
}
