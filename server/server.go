package server

import (
	"context"
	"log"
	"net/http"

	pb "github.com/diogomassis/rinha-2025/proto"
)

type Server struct {
	pb.PaymentServiceServer
}

func (s *Server) Payments(ctx context.Context, in *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	log.Printf("Endpoint '/payments' was invoked.")
	return &pb.PaymentResponse{
		Code:    http.StatusOK,
		Message: "Payment processed successfully. Correlation Id: " + in.CorrelationId,
	}, nil
}
