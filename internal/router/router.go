package router

import (
	"payment-api/internal/controllers"
	"payment-api/internal/middleware"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func New() *gin.Engine {
	controller := controllers.Controller{}
	r := gin.New()
	r.RedirectTrailingSlash = false
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipQueryString: true}), gin.CustomRecovery(func(ctx *gin.Context, recovered any) {
		ctx.Abort()
		controller.Response(ctx, nil, status.Error(codes.Internal, "服务内部错误"))
	}), middleware.RequestLogger())
	r.NoRoute(func(ctx *gin.Context) {
		controller.Response(ctx, nil, status.Error(codes.NotFound, "接口不存在"))
	})

	r.GET("/health", controller.Health)

	// 对外接口
	payC := controllers.PaymentController{}
	openPay := r.Group("/open-pay/v1")
	openPay.POST("/create/order", payC.CreateOrder)

	// 微信
	public := r.Group("/payment-api/v1")
	wechatG := public.Group("/wechat")
	wechatG.Any("/pay/notify/:merchant_account", payC.WxPayNotify)
	wechatG.POST("/reissue/pay/notify", payC.ReissueWxPayNotify)
	wechatG.POST("/auth/login", controller.WechatLogin)

	// 商户接口
	merchant := r.Group("/merchant-api/v1")
	merchant.Use(middleware.Auth(controller.Response, true))
	merchant.POST("/create/order", payC.CreateOrder)

	// 管理端接口
	admin := public.Group("/admin-api/v1")
	//admin.Use(middleware.AdminAuth(controller.Response))
	admin.POST("/create/api-keys", controller.GenerateAPIKey)

	return r
}
