package controllers

import (
	healthv1 "payment-api/api/health/v1"
	"payment-api/internal/service"

	"github.com/gin-gonic/gin"
)

func (c Controller) Health(ctx *gin.Context) {
	var health service.Health
	resp, err := health.Check(ctx.Request.Context(), &healthv1.HealthRequest{})
	c.Response(ctx, resp, err)
}
