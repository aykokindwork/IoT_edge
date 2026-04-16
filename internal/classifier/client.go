package classifier

import (
	"context"
	"fmt"
	"time"

	pb "edge-gateway/proto/classifier"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client — gRPC клиент к Python ML сервису
type Client struct {
	conn   *grpc.ClientConn
	client pb.ClassifierServiceClient
}

// New создаёт подключение к Python сервису
func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc connect failed: %w", err)
	}

	return &Client{
		conn:   conn,
		client: pb.NewClassifierServiceClient(conn),
	}, nil
}

// Classify отправляет вектор признаков и получает вердикт
func (c *Client) Classify(ctx context.Context, flowID string, features []float64) (float32, string, error) {
	// конвертируем float64 → float32 для protobuf
	f32 := make([]float32, len(features))
	for i, v := range features {
		f32[i] = float32(v)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := c.client.Classify(ctx, &pb.ClassifyRequest{
		Features: f32,
		FlowId:   flowID,
	})
	if err != nil {
		return 0, "", fmt.Errorf("classify rpc failed: %w", err)
	}

	return resp.Probability, resp.Verdict, nil
}

// Close закрывает соединение
func (c *Client) Close() error {
	return c.conn.Close()
}
