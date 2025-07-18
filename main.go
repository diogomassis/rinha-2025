package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-redis/redis/v8"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pb "github.com/diogomassis/rinha-2025/proto"
	"github.com/diogomassis/rinha-2025/server"
	"github.com/diogomassis/rinha-2025/worker"
)

var addrGrpcServer string = "0.0.0.0:3030"

func main() {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
		DB:   0,
	})
	defer redisClient.Close()

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
	} else {
		log.Println("Connected to Redis successfully")
	}

	natsConn, err := nats.Connect("nats://nats:4222")
	if err != nil {
		log.Printf("Warning: NATS connection failed: %v", err)
		natsConn = nil
	} else {
		log.Println("Connected to NATS successfully")
		defer natsConn.Close()
	}

	workerPool := worker.NewWorkerPool(10, 1000, redisClient, natsConn)
	workerPool.Start()
	defer workerPool.Stop()

	paymentServer := server.NewServer(workerPool)

	go func() {
		lis, err := net.Listen("tcp", addrGrpcServer)
		if err != nil {
			panic(err)
		}
		log.Printf("gRPC server listening at %s", addrGrpcServer)

		s := grpc.NewServer()
		pb.RegisterPaymentServiceServer(s, paymentServer)
		reflection.Register(s)
		if err = s.Serve(lis); err != nil {
			panic(err)
		}
	}()

	ctx = context.Background()
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	err = pb.RegisterPaymentServiceHandlerFromEndpoint(ctx, mux, addrGrpcServer, opts)
	if err != nil {
		panic(err)
	}

	port := ":8080"
	log.Printf("HTTP gateway listening at %s", port)
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
