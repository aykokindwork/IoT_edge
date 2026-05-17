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
func (e *Engine) HandleResult(key flowtable.FlowKey, v10 []float64, v46 []float64) {
	ctx := context.Background()
	id := fmt.Sprintf("%s:%d->%s:%d", key.SrcIP, key.SrcPort, key.DstIP, key.DstPort)

	// 1. Спрашиваем локальную модель (Python через gRPC)
	prob, verdict, err := e.classifier.Classify(ctx, id, v10)
	if err != nil {
		log.Printf("Local classification error for %s: %v", id, err)
		return
	}

	log.Printf("Flow %s -> Verdict: %s (prob: %.2f)", key.SrcIP, verdict, prob)

	// 2. Логика принятия решения
	switch verdict {
	case "attack":
		// Локальная модель уверена — баним сразу
		e.blockIP(key.SrcIP)

	case "suspicious":
		// Локальная модель сомневается — шлем ПОЛНЫЙ вектор (v46) в Облако
		log.Printf("Flow from %s is suspicious. Sending 46 features to Cloud...", key.SrcIP)
		e.producer.Push(id, v46)

	case "benign":
		// Всё чисто
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
