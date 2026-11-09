// loan-server — gRPC-сервер LoanService (власне завдання теми 13).
// Запуск: go run ./cmd/loan-server [-addr :50061]
package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "libratrack/gen/loan"
	"libratrack/internal/loan"
)

func main() {
	addr := flag.String("addr", ":50061", "адреса gRPC-сервера")
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Error("listen", "err", err)
		os.Exit(1)
	}
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(logUnary(log)))
	pb.RegisterLoanServiceServer(srv, loan.New(nil))
	reflection.Register(srv) // для grpcurl; у продакшені вимикають

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		t := time.AfterFunc(5*time.Second, srv.Stop) // відкритий стрім не має блокувати зупинку
		srv.GracefulStop()
		t.Stop()
	}()

	log.Info("loan server started", "addr", lis.Addr().String())
	if err := srv.Serve(lis); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

func logUnary(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := h(ctx, req)
		log.Info("rpc", "method", info.FullMethod, "dur", time.Since(start), "err", err)
		return resp, err
	}
}
