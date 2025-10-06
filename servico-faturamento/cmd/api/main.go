package main

import (
	"log"

	"servico-faturamento/internal/config"
	"servico-faturamento/internal/consumidor"
	"servico-faturamento/internal/manipulador"
	"servico-faturamento/internal/publicador"

	"github.com/gin-gonic/gin"
)

func main() {
	// inicializar DB
	db, err := config.InicializarDB()
	if err != nil {
		log.Fatalf("Erro ao inicializar DB: %v", err)
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	// criar handlers
	handlers := &manipulador.Handlers{DB: db}

	// iniciar publicador de eventos (outbox pattern)
	if err := publicador.IniciarPublicador(db); err != nil {
		log.Fatalf("Erro ao iniciar publicador outbox: %v", err)
	}

	// iniciar consumidor RabbitMQ - CRÍTICO para Saga funcionar
	if err := consumidor.IniciarConsumidor(db, handlers); err != nil {
		log.Fatalf("ERRO CRÍTICO: Falha ao iniciar consumidor RabbitMQ: %v", err)
	}
	log.Println("✓ Consumidor RabbitMQ iniciado com sucesso")

	// setup servidor Gin
	r := gin.Default()

	// CORS simples
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// rotas API
	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		// notas
		v1.POST("/notas", handlers.CriarNota)
		v1.GET("/notas", handlers.ListarNotas)
		v1.GET("/notas/:id", handlers.BuscarNota)
		v1.POST("/notas/:id/itens", handlers.AdicionarItem)
		v1.POST("/notas/:id/imprimir", handlers.ImprimirNota)

		// solicitações
		v1.GET("/solicitacoes-impressao/:id", handlers.ConsultarStatusImpressao)
	}

	log.Println("Servidor Faturamento iniciado na porta 8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}
