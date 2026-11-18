package pay

import (
	"context"
	"net/http"
)

type Notifier interface {
	Parse(ctx context.Context, request *http.Request, data interface{}) error
	ParseWithoutTimeCheck(ctx context.Context, request *http.Request, data interface{}) error
}
