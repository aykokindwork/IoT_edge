package capture

import (
	"context"
	"edge-gateway/internal/flowtable"
	"fmt"
	"io"
	"log"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

type Capturer struct {
	handle    *pcap.Handle
	flowTable *flowtable.FlowTable
	isOffline bool
}

func NewCapturer(device string, ft *flowtable.FlowTable, pcapPath string) (*Capturer, error) {

	var isOffline bool
	var handle *pcap.Handle
	if pcapPath != "" {
		log.Printf("Capturer: Opening offline file %s", pcapPath)
		h, err := pcap.OpenOffline(pcapPath)
		if err != nil {
			return nil, fmt.Errorf("fail to open pcap file: %w", err)
		}
		handle = h
		isOffline = true
	} else {
		h, err := pcap.OpenLive(device, 65535, true, pcap.BlockForever)
		if err != nil {
			return nil, err
		}
		handle = h
	}
	// Открываем сетевой интерфейс для захвата
	// 65535 — размер буфера (snaplen), true — promiscuous mode (слушать всё)

	return &Capturer{
		handle:    handle,
		flowTable: ft,
		isOffline: isOffline,
	}, nil

}

func (c *Capturer) Start(ctx context.Context) {
	var eth layers.Ethernet
	var ip4 layers.IPv4
	var tcp layers.TCP
	var udp layers.UDP
	var payload gopacket.Payload

	// Настраиваем парсер, который будет искать только эти слои
	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet, &eth, &ip4, &tcp, &udp, &payload)
	decoded := []gopacket.LayerType{}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var data []byte
				var ci gopacket.CaptureInfo
				var err error

				if c.isOffline {
					data, ci, err = c.handle.ReadPacketData()
					if err == io.EOF {
						log.Println("Capturer: Finished processing PCAP file.")
						return
					}
				} else {
					// Zero-copy чтение: мы получаем ссылку на байты прямо из буфера сетевой карты
					data, ci, err = c.handle.ZeroCopyReadPacketData()
				}
				
				if err != nil {
					continue
				}

				err = parser.DecodeLayers(data, &decoded)
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
		}
	}()
}
