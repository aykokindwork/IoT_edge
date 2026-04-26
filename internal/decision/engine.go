package decision

import (
	"context"
	"edge-gateway/internal/consumer"
	"fmt"
	"log"

	"edge-gateway/internal/classifier"
	"edge-gateway/internal/flowtable"
	"edge-gateway/internal/producer"
)

// Engine — сердце принятия решений
type Engine struct {
	classifier *classifier.Client
	producer   *producer.Producer
}

func NewEngine(c *classifier.Client, p *producer.Producer) *Engine {
	return &Engine{
		classifier: c,
		producer:   p,
	}
}

// HandleResult — та самая функция-колбэк (ClassifyFunc)
func (e *Engine) HandleResult(key flowtable.FlowKey, vector []float64) {
	ctx := context.Background()

	// 1. Спрашиваем локальную модель (Python через gRPC)
	prob, verdict, err := e.classifier.Classify(ctx, fmt.Sprintf("%s:%d", key.SrcIP, key.SrcPort), vector)
	if err != nil {
		log.Printf("Classification error: %v", err)
		return
	}

	log.Printf("Flow %s -> Verdict: %s (prob: %.2f)", key.SrcIP, verdict, prob)

	// 2. Логика принятия решения
	switch verdict {
	case "attack":
		e.blockIP(key.SrcIP)
	case "suspicious":
		// Шлем ту самую функцию асинхронной отправки в Кафку
		e.producer.Push(fmt.Sprintf("%s:%d", key.SrcIP, key.SrcPort), vector)
	case "benign":
		// Все хорошо, ничего не делаем
	}
}

func (e *Engine) ProccessCloudVerdict(v consumer.CloudVerdict) {
	if v.Verdict == "Benign" || v.Verdict == "BenignTraffic" {
		log.Printf("Cloud verdict for %s: Clean.", v.IP)

	} else {
		log.Printf("!!! CLOUD VERDICT: IP %s is A ATTACKER. Blocking", v.IP)
	}
}

func (e *Engine) blockIP(ip string) {
	// Здесь будет логика работы с ipset/iptables
	// Для начала просто пишем в лог, имитируя действие IPS
	log.Printf("!!! [BLOCK] IP address %s added to blacklist", ip)
}
