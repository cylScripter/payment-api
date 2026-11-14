package router

import (
	"payment-api/internal/controllers"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func New() *gin.Engine {
	controller := controllers.Controller{}
	r := gin.New()
	r.RedirectTrailingSlash = false
	r.Use(gin.Logger(), gin.CustomRecovery(func(ctx *gin.Context, recovered any) {
		ctx.Abort()
		controller.Response(ctx, nil, status.Error(codes.Internal, "服务内部错误"))
	}))
	r.NoRoute(func(ctx *gin.Context) {
		controller.Response(ctx, nil, status.Error(codes.NotFound, "接口不存在"))
	})
	r.GET("/health", controller.Health)
	r.POST("/v1/auth/wechat/login", controller.WechatLogin)
	r.POST("/v1/payments/jsapi", controller.CreateOrder)
	return r
}
