package main

import (
	"context"
	"log"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pb "github.com/diogomassis/rinha-2025/proto"
)

var addrGrpcServer string = "0.0.0.0:3030"

type Server struct {
	pb.TestServiceServer
}

func (s *Server) Test(ctx context.Context, in *pb.TestRequest) (*pb.TestResponse, error) {
	log.Printf("Test function was invoked with %v\n", in)
	return &pb.TestResponse{
		Result: "Hello, World",
	}, nil
}

func main() {
	// Start gRPC server
	go func() {
		lis, err := net.Listen("tcp", addrGrpcServer)
		if err != nil {
			panic(err)
		}
		log.Printf("gRPC server listening at %s", addrGrpcServer)

		s := grpc.NewServer()
		pb.RegisterTestServiceServer(s, &Server{})
		reflection.Register(s)
		if err = s.Serve(lis); err != nil {
			panic(err)
		}
	}()

	// Start HTTP gateway
	ctx := context.Background()
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	err := pb.RegisterTestServiceHandlerFromEndpoint(ctx, mux, addrGrpcServer, opts)
	if err != nil {
		panic(err)
	}

	port := ":8081"
	log.Printf("HTTP gateway listening at %s", port)
	log.Printf("HTTP route available at: GET http://localhost%s/v1/test", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		panic(err)
	}
}
