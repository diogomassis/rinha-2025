package server

import (
	"context"
	"log"
	"net/http"

	"github.com/diogomassis/rinha-2025/internal/env"
	"github.com/diogomassis/rinha-2025/internal/models"
	pb "github.com/diogomassis/rinha-2025/internal/proto"
	"github.com/diogomassis/rinha-2025/internal/services/cache"
)

type RinhaServer struct {
	pb.PaymentServiceServer
	client *cache.RinhaRedisClient
}

func NewRinhaServer(client *cache.RinhaRedisClient) *RinhaServer {
	return &RinhaServer{
		client: client,
	}
}

func (s *RinhaServer) Payments(ctx context.Context, in *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	pendingPayment := models.NewRinhaPendingPayment(in.CorrelationId, in.Amount)
	err := s.client.AddToQueue(ctx, env.Env.InstanceName, *pendingPayment)
	if err != nil {
		log.Printf("[server][error] unable to queue payment (correlation_id=%s, amount=%.2f): %v", in.CorrelationId, in.Amount, err)
		return &pb.PaymentResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}, nil
	}

	log.Printf("[server][info] payment queued successfully (correlation_id=%s, amount=%.2f)", in.CorrelationId, in.Amount)
	return &pb.PaymentResponse{
		Code:    http.StatusCreated,
		Message: "Payment queued for processing",
	}, nil
}

func (s *RinhaServer) PaymentsSummary(ctx context.Context, in *pb.PaymentsSummaryRequest) (*pb.PaymentsSummaryResponse, error) {
	return &pb.PaymentsSummaryResponse{
		Default: &pb.ProcessorSummary{
			TotalRequests: 0,
			TotalAmount:   0,
		},
		Fallback: &pb.ProcessorSummary{
			TotalRequests: 0,
			TotalAmount:   0,
		},
	}, nil
}
