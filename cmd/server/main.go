package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"payment-api/internal/config"
	"payment-api/internal/initials"
	"payment-api/internal/router"
	"payment-api/internal/rpc"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if err := initials.Init(); err != nil {
		return err
	}
	defer func() {
		if err := initials.Close(); err != nil {
			log.Infof("关闭基础组件失败: %v", err)
		}
	}()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	httpListener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("监听 HTTP 端口失败: %w", err)
	}
	defer func(httpListener net.Listener) {
		err := httpListener.Close()
		if err != nil {
			log.Infof("关闭 HTTP 端口失败: %v", err)
		}
	}(httpListener)
	rpcListener, err := net.Listen("tcp", config.Get().RPC.Addr)
	if err != nil {
		return fmt.Errorf("监听 RPC 端口失败: %w", err)
	}
	defer func(rpcListener net.Listener) {
		err := rpcListener.Close()
		if err != nil {
			log.Infof("关闭 RPC 端口失败: %v", err)
		}
	}(rpcListener)

	httpServer := &http.Server{Handler: router.New()}
	defer func(httpServer *http.Server) { _ = httpServer.Close() }(httpServer)
	rpcServer := rpc.New()
	defer rpcServer.Stop()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErr := make(chan error, 2)
	go func() { serveErr <- httpServer.Serve(httpListener) }()
	go func() { serveErr <- rpcServer.Serve(rpcListener) }()
	log.Infof("HTTP listening on %s, RPC listening on %s", httpListener.Addr(), rpcListener.Addr())

	select {
	case <-ctx.Done():
	case err = <-serveErr:
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rpcStopped := make(chan struct{})
	go func() {
		rpcServer.GracefulStop()
		close(rpcStopped)
	}()
	if shutdownErr := httpServer.Shutdown(shutdownCtx); shutdownErr != nil {
		_ = httpServer.Close()
	}
	select {
	case <-rpcStopped:
	case <-shutdownCtx.Done():
		rpcServer.Stop()
	}
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, grpc.ErrServerStopped) {
		return nil
	}
	return err
}
