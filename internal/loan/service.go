// Package loan — власна реалізація LoanService (домашнє завдання теми 13):
// unary CreateLoan і серверний стрімінг WatchLoans. Сховище — у пам'яті.
package loan

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "libratrack/gen/loan"
)

const (
	DefaultDays = 14
	MaxDays     = 60
	subBuffer   = 16
)

// Service реалізує pb.LoanServiceServer.
type Service struct {
	pb.UnimplementedLoanServiceServer

	now func() time.Time

	mu    sync.Mutex
	seq   int
	loans []*pb.Loan          // усі позики в порядку створення
	subs  map[int]*subscriber // активні стріми
	next  int                 // id наступної підписки
}

type subscriber struct {
	readerID string
	ch       chan *pb.Loan
}

// New створює порожній сервіс. now можна підмінити в тестах (nil = time.Now).
func New(now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{now: now, subs: map[int]*subscriber{}}
}

// CreateLoan — unary: валідація, збереження, розсилка підписникам.
func (s *Service) CreateLoan(ctx context.Context, req *pb.CreateLoanRequest) (*pb.Loan, error) {
	if req.GetBookId() == "" || req.GetReaderId() == "" {
		return nil, status.Error(codes.InvalidArgument, "book_id and reader_id are required")
	}
	days := int(req.GetDays())
	switch {
	case days == 0:
		days = DefaultDays
	case days < 0 || days > MaxDays:
		return nil, status.Errorf(codes.InvalidArgument, "days must be in 1..%d, got %d", MaxDays, days)
	}
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}

	issued := s.now().UTC()
	s.mu.Lock()
	s.seq++
	l := &pb.Loan{
		Id:           fmt.Sprintf("loan-%d", s.seq),
		BookId:       req.GetBookId(),
		ReaderId:     req.GetReaderId(),
		Status:       pb.Loan_ACTIVE,
		IssuedAtUnix: issued.Unix(),
		DueAtUnix:    issued.AddDate(0, 0, days).Unix(),
	}
	s.loans = append(s.loans, l)
	for _, sub := range s.subs {
		if sub.readerID != l.ReaderId {
			continue
		}
		select {
		case sub.ch <- l:
		default: // повільний споживач не має блокувати створення позик
		}
	}
	s.mu.Unlock()
	return l, nil
}

// WatchLoans — server streaming: наявні позики читача, далі нові до max_events
// або до скасування клієнтом (stream.Context()).
func (s *Service) WatchLoans(req *pb.WatchLoansRequest, stream pb.LoanService_WatchLoansServer) error {
	if req.GetReaderId() == "" {
		return status.Error(codes.InvalidArgument, "reader_id is required")
	}
	if req.GetMaxEvents() < 0 {
		return status.Error(codes.InvalidArgument, "max_events must be >= 0")
	}

	// Підписуємось і знімаємо знімок під одним локом, щоб не загубити й не задублювати події.
	sub := &subscriber{readerID: req.GetReaderId(), ch: make(chan *pb.Loan, subBuffer)}
	s.mu.Lock()
	var existing []*pb.Loan
	for _, l := range s.loans {
		if l.ReaderId == sub.readerID {
			existing = append(existing, l)
		}
	}
	id := s.next
	s.next++
	s.subs[id] = sub
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, id)
		s.mu.Unlock()
	}()

	sent := int32(0)
	send := func(l *pb.Loan) (done bool, err error) {
		if err := stream.Send(l); err != nil {
			return true, err
		}
		sent++
		return req.GetMaxEvents() > 0 && sent >= req.GetMaxEvents(), nil
	}

	for _, l := range existing {
		if done, err := send(l); done {
			return err
		}
	}
	for {
		select {
		case <-stream.Context().Done():
			return status.FromContextError(stream.Context().Err()).Err()
		case l := <-sub.ch:
			if done, err := send(l); done {
				return err
			}
		}
	}
}

// Subscribers повертає число активних стрімів (для перевірки витоків у тестах).
func (s *Service) Subscribers() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subs)
}
