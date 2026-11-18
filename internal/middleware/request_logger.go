package middleware

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func RequestLogger() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var body []byte
		if ctx.Request.Body != nil {
			original := ctx.Request.Body
			data, err := ctx.GetRawData()
			body = data
			ctx.Request.Body = &requestLogBody{
				Reader: bytes.NewReader(body),
				Closer: original,
				err:    err,
			}
		}
		log.WithFields(log.Fields{
			"method": ctx.Request.Method,
			"path":   ctx.Request.URL.Path,
			"query":  ctx.Request.URL.RawQuery,
			"body":   string(body),
		}).Info("请求参数")
		ctx.Next()
	}
}

type requestLogBody struct {
	io.Reader
	io.Closer
	err error
}

func (b *requestLogBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if err == io.EOF && b.err != nil {
		err = b.err
		b.err = nil
	}
	return n, err
}
