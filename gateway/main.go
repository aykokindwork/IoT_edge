package main

import (
	"context"
	"edge-gateway/internal/classifier"
	decision "edge-gateway/internal/desicion"
	"edge-gateway/internal/flowtable"
	"edge-gateway/internal/producer"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cls, err := classifier.New("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to init classifier: %v", err)
	}
	defer cls.Close()

	prod := producer.New([]string{"localhost:9092"}, "suspicious-flows")
	// Запускаем воркер Кафки в фоне
	prod.StartWorker(ctx)

	// 4. Создаем Decision Engine (Мозги)
	engine := decision.NewEngine(cls, prod)

	ft := flowtable.New(30 * time.Second)

	// Запускаем очистку таблицы.
	// Передаем метод engine.HandleResult как колбэк!
	ft.StartCleanup(ctx, engine.HandleResult)

	log.Println("Gateway is running. Press Ctrl+C to stop.")

	// ТУТ БУДЕТ ЗАХВАТ ПАКЕТОВ (gopacket)
	// Пока что просто имитируем работу: ждем сигнала завершения
	<-ctx.Done()

	log.Println("--- Shutting down gracefully... ---")

	// Даем немного времени горутинам завершить отправку
	// В реальном проде тут используются sync.WaitGroup
	time.Sleep(2 * time.Second)
	log.Println("Gateway stopped.")
}
