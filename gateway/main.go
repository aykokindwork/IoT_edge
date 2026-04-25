package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"edge-gateway/internal/capture" // Добавили
	"edge-gateway/internal/classifier"
	"edge-gateway/internal/decision" // Исправил опечатку в названии
	"edge-gateway/internal/flowtable"
	"edge-gateway/internal/producer"
)

var topic = "suspicious-flows"

func main() {
	// 1. Управление завершением
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("--- IoT Edge Gateway starting ---")

	cls, err := classifier.New("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to init classifier: %v", err)
	}
	defer cls.Close()

	kafkaAddr := os.Getenv("KAFKA_BROKERS")
	if kafkaAddr == "" {
		panic("Check the kafka_address in .env")
	}

	prod := producer.New([]string{kafkaAddr}, topic)
	prod.StartWorker()

	// 3. Инициализация логики
	engine := decision.NewEngine(cls, prod)
	ft := flowtable.New(30 * time.Second)

	// Запускаем фоновую очистку таблицы
	ft.StartCleanup(ctx, engine.HandleResult)

	// 4. Инициализация захвата пакетов
	// "eth0" — это пример для Linux.
	// Если ты на Windows, тебе нужно узнать имя своего интерфейса.
	// Для теста можно попробовать "\\Device\\NPF_..." или просто пропустить этот этап,
	// если будешь запускать сразу в Docker (там почти всегда eth0)
	pathPCAPFile := os.Getenv("PCAP_FILE_PATH")
	cap, err := capture.NewCapturer("eth0", ft, pathPCAPFile)
	if err != nil {
		log.Printf("WARNING: Failed to init capturer: %v. Running without live capture.", err)
	} else {
		cap.Start(ctx)
	}

	log.Println("Gateway is fully operational. Press Ctrl+C to stop.")

	// Ждем сигнала от ОС
	<-ctx.Done()

	log.Println("--- Shutting down gracefully... ---")

	prod.Close()

	log.Println("Gateway stopped cleanly.")
}
