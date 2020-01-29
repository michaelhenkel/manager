package server

import (
	"bytes"
	"context"
	pb "github.com/michaelhenkel/fabricmanager/runner/protos"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
)

type Server struct {
	Log *log.Logger
}

type InterfaceServer struct {
	pb.UnimplementedInterfaceServer
}

var (
	buf    bytes.Buffer
	logger = log.New(&buf, "INFO: ", log.Lshortfile)

	infof = func(info string) {
		logger.Output(2, info)
	}
)

func (r *Server) Run(socket string) error {

	r.Log.Println("Request for Device Controller")

	if _, err := os.Stat(socket); err == nil {
		r.Log.Println("socket exists, removing it")
		err = os.Remove(socket)
	}
	lis, err := net.Listen("unix", socket)
	if err != nil {
		r.Log.Fatal(err, "failed to listen")
	}
	s := grpc.NewServer()

	pb.RegisterInterfaceServer(s, &InterfaceServer{})

	if err := s.Serve(lis); err != nil {
		r.Log.Fatal(err, "failed to serve")
	}

	return nil
}

// Create implements
func (s *InterfaceServer) Create(ctx context.Context, in *pb.Intf) (*pb.CreateResult, error) {
	createResult := &pb.CreateResult{}
	return createResult, nil
}
