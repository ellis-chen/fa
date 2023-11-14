package audit

import (
	"context"

	pb "github.com/ellis-chen/fa/third_party/audit/v1"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client interface {
	pb.EventServiceClient
}

type client struct {
	pb.EventServiceClient
}

func New(addr string) Client {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		conn = nil
	}
	logrus.Debugf("Connected to audit service at %s", addr)
	c := &client{
		pb.NewEventServiceClient(conn),
	}

	return c
}

type NullClient struct {
}

func NewNullClient() Client {
	return &NullClient{}
}

func (c *NullClient) SendEvent(ctx context.Context, evt interface{}) error {
	return nil
}

func (c *NullClient) Create(ctx context.Context, in *pb.CreateEventRequest, opts ...grpc.CallOption) (*pb.CreateEventResponse, error) {
	return nil, nil
}

// List events by the given paging request and filter and sort by event creation time in descending order.
func (c *NullClient) List(ctx context.Context, in *pb.ListEventRequest, opts ...grpc.CallOption) (*pb.ListEventResponse, error) {
	return nil, nil
}
