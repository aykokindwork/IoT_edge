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

func main() {
	// 1. Управление завершением
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("--- IoT Edge Gateway starting ---")

	// 2. Инициализация внешних связей
	// ВНИМАНИЕ: Если тестишь без докера, оставь localhost. В докере поменяем на имена сервисов.
	cls, err := classifier.New("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to init classifier: %v", err)
	}
	defer cls.Close()

	prod := producer.New([]string{"192.168.1.10:9092"}, "suspicious-flows")
	prod.StartWorker(ctx)

	// 3. Инициализация логики
	engine := decision.NewEngine(cls, prod)
	ft := flowtable.New(30 * time.Second)

	// Запускаем фоновую очистку таблицы
	ft.StartCleanup(ctx, engine.HandleResult)

	// 4. Инициализация захвата пакетов
	// "eth0" — это пример для Linux.
	// Если ты на Windows, тебе нужно узнать имя своего интерфейса.
	// Для теста можно попробовать "\\Device\\NPF_..." или просто пропустить этот этап,
	// если будешь запускать сразу в Docker (там почти всегда eth0).
	cap, err := capture.NewCapturer("eth0", ft)
	if err != nil {
		log.Printf("WARNING: Failed to init capturer: %v. Running without live capture.", err)
	} else {
		cap.Start(ctx)
	}

	log.Println("Gateway is fully operational. Press Ctrl+C to stop.")

	// Ждем сигнала от ОС
	<-ctx.Done()

	log.Println("--- Shutting down gracefully... ---")

	// Важный момент: даем время воркерам дочитать данные из каналов
	time.Sleep(2 * time.Second)
	log.Println("Gateway stopped.")
}
