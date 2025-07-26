package server

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/diogomassis/rinha-2025/internal/models"
	pb "github.com/diogomassis/rinha-2025/internal/proto"
	"github.com/diogomassis/rinha-2025/internal/services/cache"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RinhaServer struct {
	pb.PaymentServiceServer
	redisQueue       *cache.RinhaRedisQueueService
	redisPersistence *cache.RinhaRedisPersistenceService
}

func NewRinhaServer(redisQueue *cache.RinhaRedisQueueService, redisPersistence *cache.RinhaRedisPersistenceService) *RinhaServer {
	return &RinhaServer{
		redisQueue:       redisQueue,
		redisPersistence: redisPersistence,
	}
}

func (s *RinhaServer) Payments(ctx context.Context, in *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	pendingPayment := models.NewRinhaPendingPayment(in.CorrelationId, in.Amount)
	err := s.redisQueue.AddToQueue(ctx, *pendingPayment)
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
	var from, to time.Time
	var err error

	if in.From != "" {
		fromStr := in.From
		if !strings.HasSuffix(fromStr, "Z") {
			fromStr += "Z"
		}
		from, err = time.Parse(time.RFC3339Nano, fromStr)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid 'from' date format - must be ISO UTC format")
		}
	}
	if in.To != "" {
		toStr := in.To
		if !strings.HasSuffix(toStr, "Z") {
			toStr += "Z"
		}
		to, err = time.Parse(time.RFC3339Nano, toStr)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid 'to' date format - must be ISO UTC format")
		}
	}

	summary, err := s.redisPersistence.Get(ctx, from, to)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to fetch payments summary")
	}

	return &pb.PaymentsSummaryResponse{
		Default: &pb.ProcessorSummary{
			TotalRequests: summary.Default.TotalRequests,
			TotalAmount:   summary.Default.TotalAmount,
		},
		Fallback: &pb.ProcessorSummary{
			TotalRequests: summary.Fallback.TotalRequests,
			TotalAmount:   summary.Fallback.TotalAmount,
		},
	}, nil
}
