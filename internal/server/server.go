package server

import (
	pb "github.com/diogomassis/rinha-2025/internal/proto"
)

type RinhaServer struct {
	pb.PaymentServiceServer
}

func NewRinhaServer() *RinhaServer {
	return &RinhaServer{}
}
