package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Controller struct{}

func (Controller) Response(ctx *gin.Context, resp proto.Message, err error) {
	httpCode := http.StatusOK
	code := codes.OK
	message := "ok"
	data := json.RawMessage(`{}`)
	if err == nil && resp != nil {
		encoded, marshalErr := (protojson.MarshalOptions{EmitUnpopulated: true}).Marshal(resp)
		if marshalErr != nil {
			err = status.Error(codes.Internal, "响应序列化失败")
		} else {
			data = encoded
		}
	}
	if err != nil {
		st := status.Convert(err)
		code = st.Code()
		message = st.Message()
		switch st.Code() {
		case codes.Canceled:
			httpCode = 499
		case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
			httpCode = http.StatusBadRequest
		case codes.NotFound:
			httpCode = http.StatusNotFound
		case codes.AlreadyExists, codes.Aborted:
			httpCode = http.StatusConflict
		case codes.PermissionDenied:
			httpCode = http.StatusForbidden
		case codes.Unauthenticated:
			httpCode = http.StatusUnauthorized
		case codes.ResourceExhausted:
			httpCode = http.StatusTooManyRequests
		case codes.DeadlineExceeded:
			httpCode = http.StatusGatewayTimeout
		case codes.Unimplemented:
			httpCode = http.StatusNotImplemented
		case codes.Unavailable:
			httpCode = http.StatusServiceUnavailable
		default:
			httpCode = http.StatusInternalServerError
		}
	}
	ctx.JSON(httpCode, gin.H{"code": code, "message": message, "data": data})
}
