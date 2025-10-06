package publicador

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"servico-faturamento/internal/dominio"

	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

type PublicadorOutbox struct {
	DB *gorm.DB
}

func IniciarPublicador(db *gorm.DB) error {
	pub := &PublicadorOutbox{DB: db}

	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://admin:admin123@rabbitmq:5672/"
	}

	var conn *amqp.Connection
	var err error

	for tentativa := 1; tentativa <= 30; tentativa++ {
		conn, err = amqp.Dial(rabbitURL)
		if err == nil {
			break
		}
		log.Printf("[outbox] tentativa=%d falha ao conectar RabbitMQ: %v", tentativa, err)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("falha ao conectar RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("falha ao abrir channel: %w", err)
	}

	if err := ch.ExchangeDeclare("faturamento-eventos", "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("falha ao declarar exchange: %w", err)
	}

	log.Println("[outbox] conectado ao RabbitMQ e pronto para publicar")
	go pub.processar(ch)
	return nil
}

func (p *PublicadorOutbox) processar(ch *amqp.Channel) {
	ctx := context.Background()

	for {
		var eventos []dominio.EventoOutbox
		if err := p.DB.Where("data_publicacao IS NULL").Order("id").Limit(20).Find(&eventos).Error; err != nil {
			log.Printf("[outbox] erro ao carregar eventos pendentes: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		if len(eventos) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}

		for _, evt := range eventos {
			msgID := strconv.FormatInt(evt.ID, 10)

			publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := ch.PublishWithContext(
				publishCtx,
				"faturamento-eventos",
				evt.TipoEvento,
				false,
				false,
				amqp.Publishing{
					MessageId:   msgID,
					ContentType: "application/json",
					Timestamp:   evt.DataOcorrencia,
					Body:        []byte(evt.Payload),
				},
			)
			cancel()

			if err != nil {
				log.Printf("[outbox] erro ao publicar id=%d tipo=%s : %v", evt.ID, evt.TipoEvento, err)
				continue
			}

			if err := p.DB.Model(&dominio.EventoOutbox{}).
				Where("id = ?", evt.ID).
				Update("data_publicacao", time.Now()).Error; err != nil {
				log.Printf("[outbox] publicado id=%d, mas falhou ao atualizar data_publicacao: %v", evt.ID, err)
				continue
			}

			log.Printf("[outbox] evento publicado id=%d tipo=%s", evt.ID, evt.TipoEvento)
		}
	}
}
