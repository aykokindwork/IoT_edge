package producer

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// SuspiciousFlow — то, что мы реально отправляем в облако
type SuspiciousFlow struct {
	FlowID    string    `json:"flow_id"`
	Features  []float64 `json:"features"`
	Timestamp int64     `json:"timestamp"`
}

type Producer struct {
	writer  *kafka.Writer
	msgChan chan SuspiciousFlow
}

// New создает нового продюсера и запускает фоновый воркер
func New(brokers []string, topic string) *Producer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireNone, // Fire-and-forget на уровне Kafka
		Async:        true,              // Внутренняя асинхронность библиотеки
	}

	p := &Producer{
		writer:  w,
		msgChan: make(chan SuspiciousFlow, 1000), // Буфер на 1000 сообщений
	}

	return p
}

// StartWorker — фоновая горутина для чтения из канала и отправки
func (p *Producer) StartWorker(ctx context.Context) {

	go func() {

		for {
			select {
			case <-ctx.Done():
				p.writer.Close()
				return

			case msg := <-p.msgChan:
				// Сериализуем в JSON для облака
				b, err := json.Marshal(msg)
				if err != nil {
					log.Printf("json marshal error: %v", err)
					continue
				}

				kmsg := kafka.Message{
					Key:   []byte(msg.FlowID),
					Value: b,
				}

				// Пишем в кафку. Таймаут нужен чтобы воркер не завис навсегда
				ctxWrite, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				err = p.writer.WriteMessages(ctxWrite, kmsg)
				cancel()

				if err != nil {
					log.Printf("kafka write error (dropping msg): %v", err)
				}
			}
		}
	}()
}

func (p *Producer) Push(flowID string, vector []float64) {
	msg := SuspiciousFlow{
		FlowID:    flowID,
		Features:  vector,
		Timestamp: time.Now().Unix(),
	}

	select {
	case p.msgChan <- msg:
		p.msgChan <- msg

	default:
		log.Println("kafka buffer full, dropping flow")
	}
}
