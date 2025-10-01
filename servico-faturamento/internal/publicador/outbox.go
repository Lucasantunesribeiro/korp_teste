package publicador

import (
	"fmt"
	"log"
	"os"
	"time"

	"servico-faturamento/internal/dominio"

	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

// IniciarPublicadorOutbox inicia worker em background pra publicar eventos pendentes no RabbitMQ
// Roda num loop infinito verificando outbox a cada 2 segundos
func IniciarPublicadorOutbox(db *gorm.DB) error {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://admin:admin123@rabbitmq:5672/"
	}

	log.Println("Conectando ao RabbitMQ para publicação de eventos...")

	// retry: rabbitmq demora pra subir
	var conn *amqp.Connection
	var ch *amqp.Channel
	var err error

	for tentativa := 1; tentativa <= 15; tentativa++ {
		conn, err = amqp.Dial(rabbitURL)
		if err == nil {
			ch, err = conn.Channel()
			if err == nil {
				log.Println("✓ Conectado ao RabbitMQ para publicação")
				break
			}
		}

		log.Printf("Tentativa %d/15 de conexão RabbitMQ: %v", tentativa, err)
		if tentativa < 15 {
			time.Sleep(3 * time.Second)
		}
	}

	if ch == nil {
		return fmt.Errorf("falha ao conectar RabbitMQ após 15 tentativas")
	}

	// declarar exchange (topic pra rotear eventos)
	err = ch.ExchangeDeclare(
		"faturamento-eventos", // nome
		"topic",               // tipo
		true,                  // durable
		false,                 // auto-deleted
		false,                 // internal
		false,                 // no-wait
		nil,                   // arguments
	)
	if err != nil {
		return fmt.Errorf("falha ao declarar exchange: %w", err)
	}

	// iniciar goroutine que processa outbox
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		log.Println("🚀 Publicador de eventos iniciado (polling a cada 2s)")

		for range ticker.C {
			processarEventosPendentes(db, ch)
		}
	}()

	return nil
}

func processarEventosPendentes(db *gorm.DB, ch *amqp.Channel) {
	// buscar até 10 eventos não publicados
	var eventos []dominio.EventoOutbox
	err := db.Where("data_publicacao IS NULL").
		Order("id ASC").
		Limit(10).
		Find(&eventos).Error

	if err != nil {
		log.Printf("Erro ao buscar eventos pendentes: %v", err)
		return
	}

	if len(eventos) == 0 {
		return // sem eventos pra publicar
	}

	log.Printf("Processando %d evento(s) pendente(s)...", len(eventos))

	for _, evento := range eventos {
		// tentar publicar no RabbitMQ
		err := ch.Publish(
			"faturamento-eventos",        // exchange
			evento.TipoEvento,             // routing key (ex: "Faturamento.ImpressaoSolicitada")
			false,                         // mandatory
			false,                         // immediate
			amqp.Publishing{
				MessageId:   fmt.Sprintf("faturamento-%d", evento.ID),
				ContentType: "application/json",
				Body:        []byte(evento.Payload),
				Timestamp:   evento.DataOcorrencia,
				DeliveryMode: amqp.Persistent, // durável
			},
		)

		if err != nil {
			log.Printf("❌ Erro ao publicar evento %d: %v", evento.ID, err)
			continue
		}

		// marcar como publicado
		agora := time.Now()
		if err := db.Model(&evento).Update("data_publicacao", agora).Error; err != nil {
			log.Printf("⚠️ Evento %d publicado mas falhou ao atualizar DB: %v", evento.ID, err)
		} else {
			log.Printf("✓ Evento publicado: %s (agregado=%s)", evento.TipoEvento, evento.IdAgregado)
		}
	}
}
