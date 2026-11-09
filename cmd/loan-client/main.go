// loan-client — клієнт LoanService.
//
//	go run ./cmd/loan-client create book-1 reader-1 [days]   # unary
//	go run ./cmd/loan-client -max 3 watch reader-1            # server streaming
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "libratrack/gen/loan"
)

func main() {
	addr := flag.String("addr", "localhost:50061", "адреса gRPC-сервера")
	max := flag.Int("max", 0, "watch: завершити після N подій (0 = до Ctrl+C)")
	flag.Parse()
	args := flag.Args()
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: loan-client [-addr A] [-max N] create <book> <reader> [days] | watch <reader>")
		os.Exit(2)
	}

	// insecure — лише для локальної розробки.
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial:", err)
		os.Exit(1)
	}
	defer conn.Close()
	c := pb.NewLoanServiceClient(conn)

	switch args[0] {
	case "create":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: create <book> <reader> [days]")
			os.Exit(2)
		}
		days := 0
		if len(args) > 3 {
			days, err = strconv.Atoi(args[3])
			if err != nil {
				fmt.Fprintln(os.Stderr, "days:", err)
				os.Exit(2)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second) // дедлайн на кожен unary-виклик
		defer cancel()
		l, err := c.CreateLoan(ctx, &pb.CreateLoanRequest{BookId: args[1], ReaderId: args[2], Days: int32(days)})
		if err != nil {
			fail(err)
		}
		printLoan("створено", l)
	case "watch":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		stream, err := c.WatchLoans(ctx, &pb.WatchLoansRequest{ReaderId: args[1], MaxEvents: int32(*max)})
		if err != nil {
			fail(err)
		}
		for {
			l, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				fmt.Println("потік завершено сервером (io.EOF) — все гаразд")
				return
			}
			if err != nil {
				if ctx.Err() != nil {
					fmt.Println("скасовано клієнтом")
					return
				}
				fail(err)
			}
			printLoan("подія", l)
		}
	default:
		fmt.Fprintln(os.Stderr, "невідома команда:", args[0])
		os.Exit(2)
	}
}

func printLoan(prefix string, l *pb.Loan) {
	fmt.Printf("%s: id=%s book=%s reader=%s status=%s due=%s\n", prefix, l.Id, l.BookId, l.ReaderId, l.Status,
		time.Unix(l.DueAtUnix, 0).UTC().Format("2006-01-02"))
}

func fail(err error) {
	st, _ := status.FromError(err)
	fmt.Fprintf(os.Stderr, "RPC помилка: code=%s message=%q\n", st.Code(), st.Message())
	os.Exit(1)
}
