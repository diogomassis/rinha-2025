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
	"github.com/diogomassis/rinha-2025/server"
)

var addrGrpcServer string = "0.0.0.0:3030"

func main() {
	// Start gRPC server
	go func() {
		lis, err := net.Listen("tcp", addrGrpcServer)
		if err != nil {
			panic(err)
		}
		log.Printf("gRPC server listening at %s", addrGrpcServer)

		s := grpc.NewServer()
		pb.RegisterPaymentServiceServer(s, &server.Server{})
		reflection.Register(s)
		if err = s.Serve(lis); err != nil {
			panic(err)
		}
	}()

	// Start HTTP gateway
	ctx := context.Background()
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	err := pb.RegisterPaymentServiceHandlerFromEndpoint(ctx, mux, addrGrpcServer, opts)
	if err != nil {
		panic(err)
	}

	port := ":8081"
	log.Printf("HTTP gateway listening at %s", port)
	log.Printf("HTTP route available at: POST http://localhost%s/payments", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		panic(err)
	}
}
