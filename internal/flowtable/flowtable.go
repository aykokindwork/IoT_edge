package flowtable

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/google/gopacket/layers"
)

type dualVector struct {
	v10 []float64
	v46 []float64
}

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

	// Размеры для статистики
	MinSize   int64
	MaxSize   int64
	SumSize   int64
	SumSqSize float64 // Для расчета Variance и Std

	// Тайминги
	LastPacketTime time.Time
	SumIAT         float64
	IATCount       int

	// Счетчики флагов (Counts)
	AckCount int
	SynCount int
	FinCount int
	UrgCount int
	RstCount int

	// Наличие флагов (Binary flags) - 0 или 1
	HasFin int
	HasSyn int
	HasRst int
	HasPsh int
	HasAck int
	HasEce int
	HasCwr int

	// Заголовки и прочее
	TotalHeaderLen int64
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

func (ft *FlowTable) Update(key FlowKey, pktSize int64, headerLen int64, pktTime time.Time, tcp *layers.TCP) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	stats, exists := ft.flows[key]
	if !exists {
		stats = &FlowStats{
			StartTime: pktTime,
			MinSize:   pktSize,
			MaxSize:   pktSize,
		}
		ft.flows[key] = stats
	}

	stats.LastSeen = pktTime
	stats.PacketCount++
	stats.TotalSize += pktSize
	stats.SumSize += pktSize
	stats.SumSqSize += float64(pktSize * pktSize)
	stats.TotalHeaderLen += headerLen

	// Обновляем Min/Max
	if pktSize < stats.MinSize {
		stats.MinSize = pktSize
	}
	if pktSize > stats.MaxSize {
		stats.MaxSize = pktSize
	}

	// Тайминги (IAT)
	if !stats.LastPacketTime.IsZero() {
		iat := pktTime.Sub(stats.LastPacketTime).Seconds()
		stats.SumIAT += iat
		stats.IATCount++
	}
	stats.LastPacketTime = pktTime

	// Извлекаем флаги TCP
	if tcp != nil {
		if tcp.FIN {
			stats.FinCount++
			stats.HasFin = 1
		}
		if tcp.SYN {
			stats.SynCount++
			stats.HasSyn = 1
		}
		if tcp.RST {
			stats.RstCount++
			stats.HasRst = 1
		}
		if tcp.PSH {
			stats.HasPsh = 1
		}
		if tcp.ACK {
			stats.AckCount++
			stats.HasAck = 1
		}
		if tcp.URG {
			stats.UrgCount++
		}
		if tcp.ECE {
			stats.HasEce = 1
		}
		if tcp.CWR {
			stats.HasCwr = 1
		}
	}
}

func (ft *FlowTable) buildVector46(key FlowKey, stats *FlowStats) []float64 {
	duration := stats.LastSeen.Sub(stats.StartTime).Seconds()
	if duration <= 0 {
		duration = 0.000001
	} // Защита от деления на 0

	rate := float64(stats.PacketCount) / duration
	avgSize := float64(stats.TotalSize) / float64(stats.PacketCount)

	// Расчет Standard Deviation (Std)
	variance := (stats.SumSqSize / float64(stats.PacketCount)) - (avgSize * avgSize)
	if variance < 0 {
		variance = 0
	}
	stdSize := math.Sqrt(variance)

	avgIAT := 0.0
	if stats.IATCount > 0 {
		avgIAT = stats.SumIAT / float64(stats.IATCount)
	}

	// Хелпер для портов
	isPort := func(p1, p2 uint16, target uint16) float64 {
		if p1 == target || p2 == target {
			return 1.0
		}
		return 0.0
	}

	v := make([]float64, 46)
	v[0] = duration
	v[1] = float64(stats.TotalHeaderLen)
	v[2] = float64(key.Proto)
	v[3] = duration // Duration (clone)
	v[4] = rate     // Rate
	v[5] = rate     // Srate (упрощенно)
	v[6] = 0.0      // Drate
	v[7] = float64(stats.HasFin)
	v[8] = float64(stats.HasSyn)
	v[9] = float64(stats.HasRst)
	v[10] = float64(stats.HasPsh)
	v[11] = float64(stats.HasAck)
	v[12] = float64(stats.HasEce)
	v[13] = float64(stats.HasCwr)
	v[14] = float64(stats.AckCount)
	v[15] = float64(stats.SynCount)
	v[16] = float64(stats.FinCount)
	v[17] = float64(stats.UrgCount)
	v[18] = float64(stats.RstCount)
	v[19] = isPort(key.SrcPort, key.DstPort, 80)   // HTTP
	v[20] = isPort(key.SrcPort, key.DstPort, 443)  // HTTPS
	v[21] = isPort(key.SrcPort, key.DstPort, 53)   // DNS
	v[22] = isPort(key.SrcPort, key.DstPort, 23)   // Telnet
	v[23] = isPort(key.SrcPort, key.DstPort, 25)   // SMTP
	v[24] = isPort(key.SrcPort, key.DstPort, 22)   // SSH
	v[25] = isPort(key.SrcPort, key.DstPort, 6667) // IRC
	v[26] = mapProto(key.Proto, 6)                 // TCP
	v[27] = mapProto(key.Proto, 17)                // UDP
	v[28] = isPort(key.SrcPort, key.DstPort, 67)   // DHCP
	v[29] = 0.0                                    // ARP (мы их фильтруем в capture, так что 0)
	v[30] = mapProto(key.Proto, 1)                 // ICMP
	v[31] = 0.0                                    // IPv (флаг)
	v[32] = 0.0                                    // LLC
	v[33] = float64(stats.TotalSize)               // Tot sum
	v[34] = float64(stats.MinSize)
	v[35] = float64(stats.MaxSize)
	v[36] = avgSize
	v[37] = stdSize
	v[38] = float64(stats.TotalSize) // Tot size
	v[39] = avgIAT
	v[40] = float64(stats.PacketCount)

	// Специфические метрики CIC (заполняем заглушками или базовыми расчетами)
	v[41] = avgSize                    // Magnitue (приблизительно)
	v[42] = 0.0                        // Radius
	v[43] = 0.0                        // Covariance
	v[44] = variance                   // Variance
	v[45] = float64(stats.PacketCount) // Weight

	return v
}

func mapProto(actual, target uint8) float64 {
	if actual == target {
		return 1.0
	}
	return 0.0
}

func (ft *FlowTable) buildVector10(key FlowKey, stats *FlowStats) []float64 {
	duration := stats.LastSeen.Sub(stats.StartTime).Seconds()
	if duration <= 0 {
		duration = 0.000001
	}

	avgSize := float64(stats.TotalSize) / float64(stats.PacketCount)
	avgIAT := 0.0
	if stats.IATCount > 0 {
		avgIAT = stats.SumIAT / float64(stats.IATCount)
	}

	// СТРОГО в порядке, который мы зафиксировали для первой модели:
	// flow_duration, Protocol Type, Duration, Tot sum, Min, Max, AVG, Tot size, IAT, Number
	return []float64{
		duration,                   // 0. flow_duration
		float64(key.Proto),         // 1. Protocol Type
		duration,                   // 2. Duration
		float64(stats.TotalSize),   // 3. Tot sum
		float64(stats.MinSize),     // 4. Min
		float64(stats.MaxSize),     // 5. Max
		avgSize,                    // 6. AVG
		float64(stats.TotalSize),   // 7. Tot size
		avgIAT,                     // 8. IAT
		float64(stats.PacketCount), // 9. Number
	}
}

type ClassifyFunc func(key FlowKey, v10 []float64, v46 []float64)

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
	// 1. now — это просто текущее время. Относительно него мы считаем,
	// не слишком ли долго "молчит" поток.
	now := time.Now()

	// 2. expired — это временная "корзина".
	// Мы сложим сюда всё, что нужно удалить из основной таблицы,
	// чтобы потом обработать это БЕЗ блокировки (mutex).
	expired := make(map[FlowKey]dualVector)

	ft.mu.Lock() // Закрываем замок — сейчас мы работаем с основной мапой
	for key, stats := range ft.flows {
		// Проверяем: если время с последнего пакета больше таймаута (например, 30с)
		if now.Sub(stats.LastSeen) > ft.timeout {

			// Собираем векторы признаков, пока у нас есть доступ к stats
			v10 := ft.buildVector10(key, stats)
			v46 := ft.buildVector46(key, stats)

			// Кладем их в нашу временную "корзину"
			expired[key] = dualVector{
				v10: v10,
				v46: v46,
			}

			// Удаляем поток из основной таблицы (чистим память)
			delete(ft.flows, key)
		}
	}
	ft.mu.Unlock() // ОТПУСКАЕМ ЗАМОК. Теперь захват пакетов снова работает.

	// 3. Теперь, когда замок открыт, мы спокойно проходим по нашей "корзине"
	// и вызываем функцию классификации (которая пойдет в gRPC и Кафку).
	for key, vectors := range expired {
		// vectors.v10 и vectors.v46 — это те данные, что мы сохранили выше
		fn(key, vectors.v10, vectors.v46)
	}
}
