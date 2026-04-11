package flowtable

import (
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
	SumSize int64 // AVG = SumSize / PacketCount

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
			SumSize:        pktSize,
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
	stats.SumSize += pktSize

	if pktSize < stats.MinSize {
		stats.MinSize = pktSize
	}

	if pktSize > stats.MaxSize {
		stats.MaxSize = pktSize
	}

	iat := pktTime.Sub(stats.LastPacketTime).Seconds()
	stats.LastPacketTime = pktTime
	stats.SumIAT += iat
	stats.IATCount++

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
