package server

import (
	"context"
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
	mainQueueChannel chan<- models.RinhaPendingPayment
}

func NewRinhaServer(redisQueue *cache.RinhaRedisQueueService, redisPersistence *cache.RinhaRedisPersistenceService, mainQueueChannel chan<- models.RinhaPendingPayment) *RinhaServer {
	return &RinhaServer{
		redisQueue:       redisQueue,
		redisPersistence: redisPersistence,
		mainQueueChannel: mainQueueChannel,
	}
}

func (s *RinhaServer) Payments(ctx context.Context, in *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	pendingPayment := models.NewRinhaPendingPayment(in.CorrelationId, in.Amount)
	select {
	case s.mainQueueChannel <- *pendingPayment:
		return &pb.PaymentResponse{
			Code:    http.StatusCreated,
			Message: "payment queued for processing",
		}, nil
	default:
		return nil, status.Error(codes.ResourceExhausted, "server is busy, please try again later")
	}
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
