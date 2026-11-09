package loan

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "libratrack/gen/loan"
)

var fixedNow = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

// start піднімає справжній gRPC-сервер на випадковому порту.
func start(t *testing.T) (pb.LoanServiceClient, *Service) {
	t.Helper()
	svc := New(func() time.Time { return fixedNow })
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	pb.RegisterLoanServiceServer(srv, svc)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return pb.NewLoanServiceClient(conn), svc
}

func TestCreateLoan_OK(t *testing.T) {
	c, _ := start(t)
	l, err := c.CreateLoan(context.Background(), &pb.CreateLoanRequest{BookId: "b1", ReaderId: "r1", Days: 7})
	require.NoError(t, err)
	assert.Equal(t, "loan-1", l.Id)
	assert.Equal(t, pb.Loan_ACTIVE, l.Status)
	assert.Equal(t, fixedNow.Unix(), l.IssuedAtUnix)
	assert.Equal(t, fixedNow.AddDate(0, 0, 7).Unix(), l.DueAtUnix)
}

func TestCreateLoan_DefaultDays(t *testing.T) {
	c, _ := start(t)
	l, err := c.CreateLoan(context.Background(), &pb.CreateLoanRequest{BookId: "b1", ReaderId: "r1"})
	require.NoError(t, err)
	assert.Equal(t, fixedNow.AddDate(0, 0, DefaultDays).Unix(), l.DueAtUnix)
}

func TestCreateLoan_InvalidArgument(t *testing.T) {
	c, _ := start(t)
	for name, req := range map[string]*pb.CreateLoanRequest{
		"no book":    {ReaderId: "r1"},
		"no reader":  {BookId: "b1"},
		"days > max": {BookId: "b1", ReaderId: "r1", Days: MaxDays + 1},
		"days < 0":   {BookId: "b1", ReaderId: "r1", Days: -3},
	} {
		_, err := c.CreateLoan(context.Background(), req)
		assert.Equal(t, codes.InvalidArgument, status.Code(err), name)
	}
}

func TestWatchLoans_ExistingThenLiveThenEOF(t *testing.T) {
	c, _ := start(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.CreateLoan(ctx, &pb.CreateLoanRequest{BookId: "b1", ReaderId: "r1"})
	require.NoError(t, err)
	_, err = c.CreateLoan(ctx, &pb.CreateLoanRequest{BookId: "bx", ReaderId: "other"}) // чужий читач
	require.NoError(t, err)

	stream, err := c.WatchLoans(ctx, &pb.WatchLoansRequest{ReaderId: "r1", MaxEvents: 2})
	require.NoError(t, err)

	first, err := stream.Recv() // наявна позика
	require.NoError(t, err)
	assert.Equal(t, "b1", first.BookId)

	_, err = c.CreateLoan(ctx, &pb.CreateLoanRequest{BookId: "b2", ReaderId: "r1"}) // жива подія
	require.NoError(t, err)
	second, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "b2", second.BookId)

	_, err = stream.Recv() // досягнуто max_events
	assert.True(t, errors.Is(err, io.EOF), "очікується io.EOF, отримано %v", err)
}

func TestWatchLoans_InvalidArgument(t *testing.T) {
	c, _ := start(t)
	stream, err := c.WatchLoans(context.Background(), &pb.WatchLoansRequest{})
	require.NoError(t, err)
	_, err = stream.Recv()
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestWatchLoans_ClientCancelReleasesSubscription(t *testing.T) {
	c, svc := start(t)
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := c.WatchLoans(ctx, &pb.WatchLoansRequest{ReaderId: "r1"})
	require.NoError(t, err)
	require.Eventually(t, func() bool { return svc.Subscribers() == 1 }, 2*time.Second, 10*time.Millisecond)

	cancel()
	_, err = stream.Recv()
	assert.Equal(t, codes.Canceled, status.Code(err))
	assert.Eventually(t, func() bool { return svc.Subscribers() == 0 }, 2*time.Second, 10*time.Millisecond,
		"підписка має бути звільнена після скасування клієнтом")
}
