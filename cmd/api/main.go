package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/diogomassis/rinha-2025/internal/env"
	"github.com/diogomassis/rinha-2025/internal/models"
	pb "github.com/diogomassis/rinha-2025/internal/proto"
	"github.com/diogomassis/rinha-2025/internal/server"
	"github.com/diogomassis/rinha-2025/internal/services/cache"
	"github.com/diogomassis/rinha-2025/internal/services/health"
	"github.com/diogomassis/rinha-2025/internal/services/orchestrator"
	"github.com/diogomassis/rinha-2025/internal/services/processor"
	"github.com/diogomassis/rinha-2025/internal/services/worker"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

var addr string = "0.0.0.0:3030"

func main() {
	env.Load()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	redisConn := redis.NewClient(&redis.Options{
		Addr:         env.Env.RedisAddr,
		Password:     "",
		DB:           0,
		PoolSize:     200,
		MinIdleConns: 20,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Second,
		PoolTimeout:  10 * time.Second,
	})
	if pong, err := redisConn.Ping(ctx).Result(); err != nil {
		log.Fatal().Err(err).Msg("Redis ping failed")
	} else {
		log.Info().Str("pong", pong).Msg("Redis connected successfully")
	}

	mainQueueChannel := make(chan models.RinhaPendingPayment, 20000)

	redisPersistence := cache.NewRinhaRedisPersistenceService(redisConn)

	processorDefault := processor.NewHTTPPaymentProcessor("default", env.Env.PaymentDefaultEndpoint)
	processorFallback := processor.NewHTTPPaymentProcessor("fallback", env.Env.PaymentFallbackEndpoint)

	healthMonitor := health.NewMonitor(processorDefault, processorFallback)
	go healthMonitor.Start()

	paymentOrchestrator := orchestrator.NewRinhaPaymentOrchestrator(healthMonitor, processorDefault, processorFallback)

	workerPool := worker.NewRinhaWorker(env.Env.WorkerConcurrency, redisPersistence, paymentOrchestrator, mainQueueChannel)
	go workerPool.Start()

	var wg sync.WaitGroup

	grpcServer := grpc.NewServer()
	pb.RegisterPaymentServiceServer(grpcServer, server.NewRinhaServer(redisPersistence, mainQueueChannel))
	reflection.Register(grpcServer)

	wg.Add(1)
	go func() {
		defer wg.Done()
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to listen")
		}
		log.Info().Str("addr", addr).Msg("gRPC server listening")
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatal().Err(err).Msg("gRPC server failed")
		}
	}()

	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if err := pb.RegisterPaymentServiceHandlerFromEndpoint(ctx, mux, addr, opts); err != nil {
		log.Fatal().Err(err).Msg("failed to register gateway")
	}
	port := ":" + env.Env.Port
	httpServer := &http.Server{Addr: port, Handler: mux}

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info().Str("instance", env.Env.InstanceName).Str("port", port).Msg("HTTP gateway listening")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("HTTP gateway failed")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("Shutdown signal received. Gracefully shutting down...")

	workerPool.Stop()
	redisConn.Close()
	grpcServer.GracefulStop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP server shutdown error")
	}

	wg.Wait()
	log.Info().Msg("Application terminated successfully.")
}
