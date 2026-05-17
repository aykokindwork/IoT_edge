package consumer

import (
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"
)

type CloudVerdict struct {
	FlowID  string `json:"flow_id"`
	Verdict string `json:"verdict"`
	IP      string `json:"ip"`
}

type Consumer struct {
	reader *kafka.Reader
}

func New(brokers []string, topic string, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			Topic:    topic,
			GroupID:  groupID,
			MinBytes: 10e3,
			MaxBytes: 10e6,
		}),
	}
}

func (c *Consumer) Start(ctx context.Context, onVerdict func(CloudVerdict)) {
	go func() {
		defer c.reader.Close()
		log.Println("cloud verdict consumer: started")

		for {
			m, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("Consumer error: %v", err)
				continue
			}
			var v CloudVerdict
			if err := json.Unmarshal(m.Value, &v); err != nil {
				log.Printf("fail to unmarshal: %v", err)
				continue
			}

			log.Printf("received clod verdict for %s, %s", v.IP, v.Verdict)
			onVerdict(v)
		}
	}()
}
