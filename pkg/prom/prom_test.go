package prom

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	pb "github.com/ellis-chen/fa/third_party/billing/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	address = "localhost:50051"
	port    = ":50051"
)

func TestUse(t *testing.T) {
	r := gin.New()
	p := NewPrometheus("gin")
	p.Use(r)
	assert.Equal(t, r.Routes()[0].Path, "/metrics")
}

// server is used to implement helloworld.GreeterServer.
type server struct {
	pb.UnimplementedEventServiceServer
}

// SayHello implements helloworld.GreeterServer
func (s *server) PutEvents(ctx context.Context, in *pb.PutEventRequest) (*pb.PutEventResponse, error) {
	log.Printf("Received: %v", in.Efs)
	return &pb.PutEventResponse{State: 2001, Msg: "success"}, nil
}

func init() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterEventServiceServer(s, &server{})
	log.Printf("server listening at %v", lis.Addr())
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	go func() {
		log.Println("Listening signals...")
		c := make(chan os.Signal, 1) // we need to reserve to buffer size 1, so the notifier are not blocked
		signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

		<-c
		s.Stop()
		log.Println("server stopped...")

	}()
}

func TestNewGrpcClient(t *testing.T) {
	t.Skip()
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewEventServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	req := pb.PutEventRequest{
		ResourceType: pb.ResourceType_RESOURCE_TYPE_STORAGE_EFS,
		Efs: &pb.Efs{
			Time:      &timestamppb.Timestamp{},
			StartTime: &timestamppb.Timestamp{},
			UserId:    10010,
			TenantId:  10202,
			Size:      1000,
			Unit:      "byte",
		},
	}
	r, err := c.PutEvent(ctx, &req)
	require.NoError(t, err)
	assert.Equal(t, int32(200), r.GetState())
}
