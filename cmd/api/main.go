package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os/signal"
	"strings"
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
	"github.com/diogomassis/rinha-2025/internal/services/requeuer"
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

	pendingPaymentChan := make(chan models.RinhaPendingPayment, 30000)
	retryPaymentChan := make(chan models.RinhaPendingPayment, 30000)
	redisPersistence := cache.NewRinhaRedisPersistenceService(redisConn)

	processorDefault := processor.NewHTTPPaymentProcessor("default", env.Env.PaymentDefaultEndpoint)
	processorFallback := processor.NewHTTPPaymentProcessor("fallback", env.Env.PaymentFallbackEndpoint)

	healthMonitor := health.NewRinhaHealthCheckerMonitor(processorDefault, processorFallback)
	go healthMonitor.Start()

	paymentOrchestrator := orchestrator.NewRinhaPaymentOrchestrator(healthMonitor, processorDefault, processorFallback)

	paymentRequeuer := requeuer.NewRinhaRequeuer(retryPaymentChan, pendingPaymentChan)
	go paymentRequeuer.Start()

	workerPool, err := worker.NewRinhaWorkerBuilder().
		WithNumWorkers(env.Env.WorkerConcurrency).
		WithRedisPersistence(redisPersistence).
		WithPaymentOrchestrator(paymentOrchestrator).
		WithPendingPaymentChannel(pendingPaymentChan).
		WithRetryPaymentChannel(retryPaymentChan).
		Build()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to build worker pool")
	}
	go workerPool.Start()

	var wg sync.WaitGroup

	grpcServer := grpc.NewServer()
	pb.RegisterPaymentServiceServer(grpcServer, server.NewRinhaServer(redisPersistence, pendingPaymentChan))
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
	httpServer := &http.Server{Addr: port, Handler: grpcHandlerFunc(grpcServer, newGatewayMux(ctx))}

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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP server shutdown error")
	}

	close(pendingPaymentChan)
	close(retryPaymentChan)

	healthMonitor.Stop()
	paymentRequeuer.Stop()
	workerPool.Wait()
	redisConn.Close()

	wg.Wait()
	log.Info().Msg("Application terminated successfully.")
}

func newGatewayMux(ctx context.Context) http.Handler {
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if err := pb.RegisterPaymentServiceHandlerFromEndpoint(ctx, mux, addr, opts); err != nil {
		log.Fatal().Err(err).Msg("Failed to register gateway")
	}
	return mux
}

func grpcHandlerFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			otherHandler.ServeHTTP(w, r)
		}
	})
}
