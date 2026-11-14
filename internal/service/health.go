package service

import (
	"context"

	healthv1 "payment-api/api/health/v1"
)

type Health struct {
	healthv1.UnimplementedHealthServiceServer
}

func (h *Health) Check(ctx context.Context, req *healthv1.HealthRequest) (*healthv1.HealthResponse, error) {
	return &healthv1.HealthResponse{Status: "ok"}, nil
}
