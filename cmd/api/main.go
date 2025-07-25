package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/diogomassis/rinha-2025/internal/env"
	pb "github.com/diogomassis/rinha-2025/internal/proto"
	"github.com/diogomassis/rinha-2025/internal/server"
	"github.com/diogomassis/rinha-2025/internal/services/cache"
	"github.com/diogomassis/rinha-2025/internal/services/health"
	"github.com/diogomassis/rinha-2025/internal/services/orchestrator"
	"github.com/diogomassis/rinha-2025/internal/services/processor"
	"github.com/diogomassis/rinha-2025/internal/services/requeuer"
	"github.com/diogomassis/rinha-2025/internal/services/worker"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

var addr string = "0.0.0.0:3030"

func main() {
	env.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	redisConn := redis.NewClient(&redis.Options{
		Addr:         env.Env.RedisAddr,
		Password:     "",
		DB:           0,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	})
	if err := redisConn.Ping(ctx); err != nil {
		log.Fatalf("[main] Redis ping failed: %v", err)
	}
	log.Printf("[main] Redis connected successfully")

	redisQueue := cache.NewRinhaRedisQueueService(redisConn)
	redisPersistence := cache.NewRinhaRedisPersistenceService(redisConn)

	processorDefault := processor.NewHTTPPaymentProcessor("default", env.Env.PaymentDefaultEndpoint, 5*time.Second)
	processorFallback := processor.NewHTTPPaymentProcessor("fallback", env.Env.PaymentFallbackEndpoint, 10*time.Second)

	healthMonitor := health.NewMonitor(processorDefault, processorFallback)
	go healthMonitor.Start()

	paymentOrchestrator := orchestrator.NewRinhaPaymentOrchestrator(healthMonitor, processorDefault, processorFallback)

	numWorkers := 50
	workerPool := worker.NewRinhaWorker(numWorkers, redisQueue, redisPersistence, paymentOrchestrator)
	go workerPool.Start()

	requeuerService := requeuer.NewRequeuer(redisConn, env.Env.InstanceName)
	go requeuerService.Start()

	var wg sync.WaitGroup

	grpcServer := grpc.NewServer()
	pb.RegisterPaymentServiceServer(grpcServer, server.NewRinhaServer(redisQueue, redisPersistence))
	reflection.Register(grpcServer)

	wg.Add(1)
	go func() {
		defer wg.Done()
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		log.Printf("gRPC server listening at %s", addr)
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if err := pb.RegisterPaymentServiceHandlerFromEndpoint(ctx, mux, addr, opts); err != nil {
		log.Fatalf("[main] failed to register gateway: %v", err)
	}
	port := ":" + env.Env.Port
	httpServer := &http.Server{Addr: port, Handler: mux}

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("[%s] HTTP gateway listening at %s", env.Env.InstanceName, port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP gateway failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("[main] Shutdown signal received. Gracefully shutting down...")

	workerPool.Stop()
	redisConn.Close()
	requeuerService.Stop()
	grpcServer.GracefulStop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	wg.Wait()
	log.Println("[main] Application terminated successfully.")
}
