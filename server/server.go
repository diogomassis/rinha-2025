package server

import (
	"context"
	"log"
	"net/http"

	pb "github.com/diogomassis/rinha-2025/proto"
	"github.com/diogomassis/rinha-2025/worker"
	"github.com/google/uuid"
)

type RinhaServer struct {
	pb.PaymentServiceServer
	workerPool *worker.RinhaWorkerPool
}

func NewRinhaServer(workerPool *worker.RinhaWorkerPool) *RinhaServer {
	return &RinhaServer{
		workerPool: workerPool,
	}
}

func (s *RinhaServer) Payments(ctx context.Context, in *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	log.Printf("Processing payment request for correlation ID: %s, amount: %.2f", in.CorrelationId, in.Amount)

	if in.CorrelationId == "" {
		return &pb.PaymentResponse{
			Code:    http.StatusBadRequest,
			Message: "CorrelationId is required",
		}, nil
	}

	if _, err := uuid.Parse(in.CorrelationId); err != nil {
		return &pb.PaymentResponse{
			Code:    http.StatusBadRequest,
			Message: "CorrelationId must be a valid UUID",
		}, nil
	}

	if in.Amount <= 0 {
		return &pb.PaymentResponse{
			Code:    http.StatusBadRequest,
			Message: "Amount must be greater than 0",
		}, nil
	}

	exists, err := s.workerPool.CheckCorrelationIdExists(ctx, in.CorrelationId)
	if err != nil {
		log.Printf("Failed to check correlation ID uniqueness for %s: %v", in.CorrelationId, err)
		return &pb.PaymentResponse{
			Code:    http.StatusInternalServerError,
			Message: "Internal server error checking request uniqueness",
		}, nil
	}

	if exists {
		return &pb.PaymentResponse{
			Code:    http.StatusConflict,
			Message: "Payment with this correlationId already processed",
		}, nil
	}

	response, err := s.workerPool.SubmitJob(ctx, in)
	if err != nil {
		log.Printf("Failed to process payment %s: %v", in.CorrelationId, err)
		return &pb.PaymentResponse{
			Code:    http.StatusInternalServerError,
			Message: "Internal server error: " + err.Error(),
		}, nil
	}

	if response.Code < 200 || response.Code >= 300 {
		response.Code = http.StatusOK
	}

	log.Printf("Payment %s processed successfully", in.CorrelationId)
	return response, nil
}

func (s *RinhaServer) PaymentsSummary(ctx context.Context, in *pb.PaymentsSummaryRequest) (*pb.PaymentsSummaryResponse, error) {
	log.Printf("Retrieving payments summary from=%s to=%s", in.From, in.To)

	summary, err := s.workerPool.GetPaymentsSummary(in.From, in.To)
	if err != nil {
		log.Printf("Failed to get payments summary: %v", err)
		return &pb.PaymentsSummaryResponse{
			Default:  &pb.ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
			Fallback: &pb.ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
		}, nil
	}

	log.Printf("Summary retrieved: default=%d requests/%.2f amount, fallback=%d requests/%.2f amount",
		summary.Default.TotalRequests, summary.Default.TotalAmount,
		summary.Fallback.TotalRequests, summary.Fallback.TotalAmount)

	return summary, nil
}
