package producer

import (
	"context"
	"encoding/json"
	"log"
	"sync"
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
	wg      sync.WaitGroup
}

// New создает нового продюсера и запускает фоновый воркер
func New(brokers []string, topic string) *Producer {
	w := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.LeastBytes{},
		RequiredAcks:           kafka.RequireNone, // Fire-and-forget на уровне Kafka
		Async:                  true,              // Внутренняя асинхронность библиотеки
		AllowAutoTopicCreation: true,
	}

	p := &Producer{
		writer:  w,
		msgChan: make(chan SuspiciousFlow, 1000), // Буфер на 1000 сообщений
	}

	return p
}

// StartWorker — фоновая горутина для чтения из канала и отправки
func (p *Producer) StartWorker() {

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer p.writer.Close()

		for msg := range p.msgChan {
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
		log.Printf("kafka succeed all messages and stop working")

	}()
}

func (p *Producer) Close() {
	close(p.msgChan)
	p.wg.Wait()
}

func (p *Producer) Push(flowID string, vector []float64) {
	msg := SuspiciousFlow{
		FlowID:    flowID,
		Features:  vector,
		Timestamp: time.Now().Unix(),
	}

	select {
	case p.msgChan <- msg:

	default:
		log.Printf("kafka buffer full, dropping flow %s", flowID)
	}
}
