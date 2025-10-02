package publicador

import (
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
		log.Printf("Tentativa %d: falha ao conectar RabbitMQ, retry em 3s...", tentativa)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("falha ao conectar RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("falha ao abrir channel: %w", err)
	}

	err = ch.ExchangeDeclare("faturamento-eventos", "topic", true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("falha ao declarar exchange: %w", err)
	}

	log.Println("✓ Publicador Outbox conectado ao RabbitMQ")
	go pub.processar(ch)
	return nil
}

func (p *PublicadorOutbox) processar(ch *amqp.Channel) {
	for {
		var eventos []dominio.EventoOutbox
		p.DB.Where("data_publicacao IS NULL").Limit(10).Find(&eventos)

		for _, evt := range eventos {
			// CORRIGIDO: int64 para string usando strconv
			msgID := strconv.FormatInt(evt.ID, 10)

			props := amqp.Publishing{
				MessageId:   msgID,
				ContentType: "application/json",
				Timestamp:   evt.DataOcorrencia,
				Body:        []byte(evt.Payload),
			}

			err := ch.Publish("faturamento-eventos", evt.TipoEvento, false, false, props)
			if err != nil {
				log.Printf("✗ Erro ao publicar evento %d: %v", evt.ID, err)
				continue
			}

			agora := time.Now()
			p.DB.Model(&evt).Update("data_publicacao", agora)
			log.Printf("✓ Evento publicado: %s (ID: %d)", evt.TipoEvento, evt.ID)
		}

		time.Sleep(2 * time.Second)
	}
}
