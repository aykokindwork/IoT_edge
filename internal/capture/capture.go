package capture

import (
	"context"

	"edge-gateway/internal/flowtable"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

type Capturer struct {
	handle    *pcap.Handle
	flowTable *flowtable.FlowTable
}

func NewCapturer(device string, ft *flowtable.FlowTable) (*Capturer, error) {
	// Открываем сетевой интерфейс для захвата
	// 65535 — размер буфера (snaplen), true — promiscuous mode (слушать всё)
	handle, err := pcap.OpenLive(device, 65535, true, pcap.BlockForever)
	if err != nil {
		return nil, err
	}

	return &Capturer{
		handle:    handle,
		flowTable: ft,
	}, nil
}

func (c *Capturer) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Zero-copy чтение: мы получаем ссылку на байты прямо из буфера сетевой карты
				data, ci, err := c.handle.ZeroCopyReadPacketData()
				if err != nil {
					continue
				}
				c.processPacket(data, ci)
			}
		}
	}()
}

func (c *Capturer) processPacket(data []byte, ci gopacket.CaptureInfo) {
	// Создаем заготовки для слоев (они переиспользуются, память не выделяется заново)
	var eth layers.Ethernet
	var ip4 layers.IPv4
	var tcp layers.TCP
	var udp layers.UDP
	var payload gopacket.Payload

	// Настраиваем парсер, который будет искать только эти слои
	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet, &eth, &ip4, &tcp, &udp, &payload)
	decoded := []gopacket.LayerType{}

	// Парсим байты пакета
	err := parser.DecodeLayers(data, &decoded)
	if err != nil {
		// Это может быть не IP пакет (например, ARP или IPv6, который мы не ждем) — просто игнорим
		return
	}

	// Нам нужны данные для FlowKey
	key := flowtable.FlowKey{}
	foundTransport := false

	// Проходим по расшифрованным слоям
	for _, typ := range decoded {
		switch typ {
		case layers.LayerTypeIPv4:
			key.SrcIP = ip4.SrcIP.String()
			key.DstIP = ip4.DstIP.String()
			key.Proto = uint8(ip4.Protocol)
		case layers.LayerTypeTCP:
			key.SrcPort = uint16(tcp.SrcPort)
			key.DstPort = uint16(tcp.DstPort)
			foundTransport = true
		case layers.LayerTypeUDP:
			key.SrcPort = uint16(udp.SrcPort)
			key.DstPort = uint16(udp.DstPort)
			foundTransport = true
		}
	}

	// Если нашли IP и транспортный уровень (порты) — обновляем таблицу
	if key.SrcIP != "" && foundTransport {
		// Используем время захвата пакета из pcap и его длину
		c.flowTable.Update(key, int64(len(data)), ci.Timestamp)
	}
}
