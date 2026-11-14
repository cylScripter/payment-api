package rpc

import (
	authv1 "payment-api/api/auth/v1"
	healthv1 "payment-api/api/health/v1"
	paymentv1 "payment-api/api/payment/v1"
	"payment-api/internal/service"

	"google.golang.org/grpc"
)

func New() *grpc.Server {
	server := grpc.NewServer()
	healthv1.RegisterHealthServiceServer(server, &service.Health{})
	authv1.RegisterAuthServiceServer(server, &service.Auth{})
	paymentv1.RegisterPaymentServiceServer(server, &service.Payment{})
	return server
}
