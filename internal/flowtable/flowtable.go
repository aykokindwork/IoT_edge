package flowtable

import (
	"context"
	"sync"
	"time"
)

// FlowKey — уникальный идентификатор сетевого соединения
type FlowKey struct {
	SrcIP   string
	DstIP   string
	SrcPort uint16
	DstPort uint16
	Proto   uint8
}

// FlowStats — накопленная статистика по одному flow
type FlowStats struct {
	StartTime   time.Time
	LastSeen    time.Time
	PacketCount int
	TotalSize   int64

	// Онлайн статистика размеров пакетов
	MinSize int64
	MaxSize int64

	// Онлайн статистика IAT
	LastPacketTime time.Time
	MinIAT         float64
	MaxIAT         float64
	SumIAT         float64 // AVG = SumIAT / (PacketCount - 1)
	IATCount       int
}

// FlowTable — потокобезопасная таблица активных flow
type FlowTable struct {
	mu      sync.RWMutex
	flows   map[FlowKey]*FlowStats
	timeout time.Duration
}

// New создаёт новую FlowTable с заданным таймаутом
func New(timeout time.Duration) *FlowTable {
	return &FlowTable{
		flows:   make(map[FlowKey]*FlowStats),
		timeout: timeout,
	}
}

func (ft *FlowTable) Update(key FlowKey, pktSize int64, pktTime time.Time) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	stats, exists := ft.flows[key]

	if !exists {
		ft.flows[key] = &FlowStats{
			StartTime:      pktTime,
			LastSeen:       pktTime,
			PacketCount:    1,
			TotalSize:      pktSize,
			MinSize:        pktSize,
			MaxSize:        pktSize,
			LastPacketTime: pktTime,
			MinIAT:         0,
			MaxIAT:         0,
			SumIAT:         0,
			IATCount:       0,
		}
		return
	}

	stats.LastSeen = pktTime
	stats.PacketCount++
	stats.TotalSize += pktSize

	if pktSize < stats.MinSize {
		stats.MinSize = pktSize
	}

	if pktSize > stats.MaxSize {
		stats.MaxSize = pktSize
	}

	iat := pktTime.Sub(stats.LastPacketTime).Seconds()
	stats.LastPacketTime = pktTime
	stats.IATCount++
	stats.SumIAT += iat

	if stats.IATCount == 1 {
		stats.MinIAT = iat
		stats.MaxIAT = iat
	} else {
		if iat < stats.MinIAT {
			stats.MinIAT = iat
		}
		if iat > stats.MaxIAT {
			stats.MaxIAT = iat
		}
	}

}

func buildVector(key FlowKey, stats *FlowStats) []float64 {
	duration := stats.LastSeen.Sub(stats.StartTime).Seconds()

	avgSize := float64(0)
	if stats.PacketCount > 0 {
		avgSize = float64(stats.TotalSize) / float64(stats.PacketCount)
	}

	avgIAT := float64(0)
	if stats.IATCount > 0 {
		avgIAT = stats.SumIAT / float64(stats.IATCount)
	}

	return []float64{
		duration,
		float64(key.Proto),
		duration,
		float64(stats.TotalSize),
		float64(stats.MinSize),
		float64(stats.MaxSize),
		avgSize,
		float64(stats.TotalSize),
		avgIAT,
		float64(stats.PacketCount),
	}
}

type ClassifyFunc func(key FlowKey, vector []float64)

func (ft *FlowTable) StartCleanup(ctx context.Context, fn ClassifyFunc) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ft.cleanup(fn)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (ft *FlowTable) cleanup(fn ClassifyFunc) {
	now := time.Now()
	// Временная структура для хранения данных вне лока
	expired := make(map[FlowKey][]float64)

	ft.mu.Lock()
	for key, stats := range ft.flows {
		if now.Sub(stats.LastSeen) > ft.timeout {
			// Собираем векторы признаков для тех, кто "протух"
			expired[key] = buildVector(key, stats)
			// Удаляем из таблицы немедленно
			delete(ft.flows, key)
		}
	}
	ft.mu.Unlock() // ОТПУСКАЕМ ЗАМОК. Теперь захват пакетов не заблокирован.

	// Теперь спокойно итерируемся по копии и делаем gRPC/Kafka вызовы
	for key, vector := range expired {
		fn(key, vector)
	}
}
