package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/diogomassis/rinha-2025/internal/env"
	pb "github.com/diogomassis/rinha-2025/internal/proto"
	"github.com/diogomassis/rinha-2025/internal/server"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	env.Load()

	ctx := context.Background()
	server := server.NewRinhaServer()
	go func() {
		lis, err := net.Listen("tcp", env.Env.GrpcAddr)
		if err != nil {
			panic(err)
		}
		log.Printf("gRPC server listening at %s", env.Env.GrpcAddr)

		s := grpc.NewServer()
		pb.RegisterPaymentServiceServer(s, server)
		reflection.Register(s)
		if err = s.Serve(lis); err != nil {
			panic(err)
		}
	}()

	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	err := pb.RegisterPaymentServiceHandlerFromEndpoint(ctx, mux, env.Env.GrpcAddr, opts)
	if err != nil {
		panic(err)
	}
	port := ":" + env.Env.BackendPort

	log.Printf("[%s] HTTP gateway listening at %s", env.Env.InstanceName, port)
	log.Printf("HTTP routes available:")
	log.Printf("  POST http://localhost%s/payments", port)
	log.Printf("  GET http://localhost%s/payments-summary", port)

	go func() {
		if err := http.ListenAndServe(port, mux); err != nil {
			panic(err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Shutting down gracefully...")
}
