package consumidor

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"servico-faturamento/internal/dominio"
	"servico-faturamento/internal/manipulador"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Consumidor struct {
	DB       *gorm.DB
	Handlers *manipulador.Handlers
}

func IniciarConsumidor(db *gorm.DB, handlers *manipulador.Handlers) error {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://admin:admin123@rabbitmq:5672/"
	}

	var conn *amqp.Connection
	var err error

	// retry de conexão
	for i := 0; i < 10; i++ {
		conn, err = amqp.Dial(rabbitURL)
		if err == nil {
			break
		}
		log.Printf("Tentativa %d: falha ao conectar RabbitMQ, retry em 3s...", i+1)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("falha ao conectar RabbitMQ após retries: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("falha ao abrir channel: %w", err)
	}

	// declarar exchange
	err = ch.ExchangeDeclare(
		"estoque-eventos", // nome
		"topic",           // tipo
		true,              // durable
		false,             // auto-deleted
		false,             // internal
		false,             // no-wait
		nil,               // arguments
	)
	if err != nil {
		return fmt.Errorf("falha ao declarar exchange: %w", err)
	}

	// declarar fila
	q, err := ch.QueueDeclare(
		"faturamento-eventos", // nome
		true,                  // durable
		false,                 // delete when unused
		false,                 // exclusive
		false,                 // no-wait
		nil,                   // arguments
	)
	if err != nil {
		return fmt.Errorf("falha ao declarar fila: %w", err)
	}

	// bind routing keys
	err = ch.QueueBind(
		q.Name,              // queue name
		"Estoque.Reservado", // routing key
		"estoque-eventos",   // exchange
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("falha ao fazer bind Reservado: %w", err)
	}

	err = ch.QueueBind(
		q.Name,                     // queue name
		"Estoque.ReservaRejeitada", // routing key
		"estoque-eventos",          // exchange
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("falha ao fazer bind Rejeitada: %w", err)
	}

	// QoS: processa 1 mensagem por vez
	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("falha ao configurar QoS: %w", err)
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack (desligado, ack manual)
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return fmt.Errorf("falha ao registrar consumer: %w", err)
	}

	log.Println("Consumidor RabbitMQ iniciado, aguardando mensagens...")

	consumidor := &Consumidor{
		DB:       db,
		Handlers: handlers,
	}

	// goroutine para processar mensagens
	go func() {
		for msg := range msgs {
			err := consumidor.ProcessarMensagem(msg)
			if err != nil {
				log.Printf("Erro ao processar mensagem: %v", err)
				msg.Nack(false, true) // requeue
			} else {
				msg.Ack(false)
			}
		}
	}()

	return nil
}

func (c *Consumidor) ProcessarMensagem(msg amqp.Delivery) error {
	idMsg := msg.MessageId
	if idMsg == "" {
		idMsg = fmt.Sprintf("%d-%s", msg.DeliveryTag, msg.RoutingKey)
	}

	log.Printf("Processando mensagem: %s (routing: %s)", idMsg, msg.RoutingKey)

	// verifica idempotência ANTES de fazer qualquer coisa
	return c.DB.Transaction(func(tx *gorm.DB) error {
		var existe dominio.MensagemProcessada
		if err := tx.Where("id_mensagem = ?", idMsg).First(&existe).Error; err == nil {
			log.Printf("Mensagem %s já processada, ignorando", idMsg)
			return nil
		}

		statusMensagem := "sucesso"

		// processar conforme routing key
		switch msg.RoutingKey {
		case "Estoque.Reservado":
			notaFechada, err := c.processarEstoqueReservado(tx, msg.Body)
			if err != nil {
				return err
			}
			if !notaFechada {
				statusMensagem = "ignorada"
			}
		case "Estoque.ReservaRejeitada":
			if err := c.processarReservaRejeitada(tx, msg.Body); err != nil {
				return err
			}
		default:
			log.Printf("Routing key desconhecida: %s", msg.RoutingKey)
			return nil
		}

		// marcar como processada
		msgProc := dominio.MensagemProcessada{
			IDMensagem:     idMsg,
			DataProcessada: time.Now(),
		}
		if err := tx.Create(&msgProc).Error; err != nil {
			return err
		}

		if statusMensagem == "sucesso" {
			log.Printf("Mensagem %s processada com sucesso", idMsg)
		} else {
			log.Printf("Mensagem %s marcada como %s", idMsg, statusMensagem)
		}
		return nil
	})
}

func (c *Consumidor) processarEstoqueReservado(tx *gorm.DB, body []byte) (bool, error) {
	var evento struct {
		NotaID     string `json:"notaId"`
		ProdutoID  string `json:"produtoId"`
		Quantidade int    `json:"quantidade"`
	}

	if err := json.Unmarshal(body, &evento); err != nil {
		return false, fmt.Errorf("falha ao fazer unmarshal: %w", err)
	}

	notaID, err := uuid.Parse(evento.NotaID)
	if err != nil {
		return false, fmt.Errorf("notaId inválido: %w", err)
	}

	log.Printf("Estoque reservado para nota %s, fechando nota...", notaID)

	// fechar nota usando handler (que já tem lock pessimista)
	// mas precisamos usar a mesma transação
	var nota dominio.NotaFiscal
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Itens").
		First(&nota, "id = ?", notaID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Nota %s não encontrada; evento será marcado como ignorado", notaID)
			return false, nil
		}
		return false, fmt.Errorf("falha ao buscar nota: %w", err)
	}

	if err := nota.Fechar(); err != nil {
		return false, fmt.Errorf("falha ao fechar nota: %w", err)
	}

	if err := tx.Save(&nota).Error; err != nil {
		return false, fmt.Errorf("falha ao salvar nota: %w", err)
	}

	// atualizar solicitação
	agora := time.Now()
	if err := tx.Model(&dominio.SolicitacaoImpressao{}).
		Where("nota_id = ? AND status = ?", notaID, "PENDENTE").
		Updates(map[string]interface{}{
			"status":          "CONCLUIDA",
			"data_conclusao": agora,
		}).Error; err != nil {
		return false, fmt.Errorf("falha ao atualizar solicitação: %w", err)
	}

	log.Printf("Nota %s fechada com sucesso", notaID)
	return true, nil
}

func (c *Consumidor) processarReservaRejeitada(tx *gorm.DB, body []byte) error {
	var evento struct {
		NotaID string `json:"notaId"`
		Motivo string `json:"motivo"`
	}

	if err := json.Unmarshal(body, &evento); err != nil {
		return fmt.Errorf("falha ao fazer unmarshal: %w", err)
	}

	notaID, err := uuid.Parse(evento.NotaID)
	if err != nil {
		return fmt.Errorf("notaId inválido: %w", err)
	}

	log.Printf("Reserva rejeitada para nota %s: %s", notaID, evento.Motivo)

	// marcar solicitação como FALHOU
	if err := tx.Model(&dominio.SolicitacaoImpressao{}).
		Where("nota_id = ? AND status = ?", notaID, "PENDENTE").
		Updates(map[string]interface{}{
			"status":         "FALHOU",
			"mensagem_erro": evento.Motivo,
		}).Error; err != nil {
		return fmt.Errorf("falha ao atualizar solicitação: %w", err)
	}

	log.Printf("Solicitação marcada como FALHOU para nota %s", notaID)
	return nil
}
