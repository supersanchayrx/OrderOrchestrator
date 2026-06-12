package order

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

type recordingPublisher struct {
	events []OrderStateChangedEvent
}

func (p *recordingPublisher) Publish(_ context.Context, event OrderStateChangedEvent) error {
	p.events = append(p.events, event)
	return nil
}

type noopBroadcaster struct{}

func (noopBroadcaster) Broadcast(string, any) {}

func newTestService() (*Service, *MemoryRepository, *recordingPublisher) {
	repo := NewMemoryRepository()
	publisher := &recordingPublisher{}
	service := NewService(repo, publisher, newMemoryIdemForTest(), noopBroadcaster{}, slog.Default())
	return service, repo, publisher
}

func TestServiceTransitionRecordsHistoryAndPublishesEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service, _, publisher := newTestService()
	created, err := service.Create(ctx, CreateOrderRequest{
		CustomerID:  "customer-1",
		TotalAmount: "499.00",
		Items: []CreateOrderItem{{
			SKUID:     "sku-1",
			Quantity:  1,
			UnitPrice: "499.00",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := service.Transition(ctx, created.ID, TransitionRequest{
		TargetState: StatePaymentPending,
		TriggeredBy: "USER",
	}, "idem-1")
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != StatePaymentPending {
		t.Fatalf("state = %s, want %s", updated.State, StatePaymentPending)
	}
	if updated.Version != 1 {
		t.Fatalf("version = %d, want 1", updated.Version)
	}

	history, err := service.History(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("history length = %d, want 2", len(history))
	}
	if history[1].FromState == nil || *history[1].FromState != StatePlaced || history[1].ToState != StatePaymentPending {
		t.Fatalf("unexpected transition history entry: %+v", history[1])
	}

	if len(publisher.events) != 1 {
		t.Fatalf("published events = %d, want 1", len(publisher.events))
	}
	if publisher.events[0].From != StatePlaced || publisher.events[0].To != StatePaymentPending {
		t.Fatalf("unexpected event: %+v", publisher.events[0])
	}
}

func TestServiceTransitionRejectsInvalidMove(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service, _, _ := newTestService()
	created, err := service.Create(ctx, CreateOrderRequest{
		CustomerID:  "customer-1",
		TotalAmount: "499.00",
		Items:       []CreateOrderItem{{SKUID: "sku-1", Quantity: 1, UnitPrice: "499.00"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Transition(ctx, created.ID, TransitionRequest{
		TargetState: StateDelivered,
		TriggeredBy: "TEST",
	}, "idem-invalid")
	var invalid *ErrInvalidTransition
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v, want ErrInvalidTransition", err)
	}
}

func TestServiceTransitionIsIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service, _, publisher := newTestService()
	created, err := service.Create(ctx, CreateOrderRequest{
		CustomerID:  "customer-1",
		TotalAmount: "499.00",
		Items:       []CreateOrderItem{{SKUID: "sku-1", Quantity: 1, UnitPrice: "499.00"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.Transition(ctx, created.ID, TransitionRequest{
		TargetState: StatePaymentPending,
		TriggeredBy: "USER",
	}, "same-key")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Transition(ctx, created.ID, TransitionRequest{
		TargetState: StatePaymentPending,
		TriggeredBy: "USER",
	}, "same-key")
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != second.Version {
		t.Fatalf("second call version = %d, want cached version %d", second.Version, first.Version)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("published events = %d, want 1", len(publisher.events))
	}
}

func TestTimeoutTransition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service, repo, _ := newTestService()
	created, err := service.Create(ctx, CreateOrderRequest{
		CustomerID:  "customer-1",
		TotalAmount: "499.00",
		Items:       []CreateOrderItem{{SKUID: "sku-1", Quantity: 1, UnitPrice: "499.00"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.InsertTimeout(ctx, Timeout{
		OrderID:       created.ID,
		ExpectedState: StatePlaced,
		Deadline:      time.Now().UTC().Add(-time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := service.TimeoutTransition(ctx, created.ID, StatePlaced)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != StateCancelled {
		t.Fatalf("state = %s, want %s", updated.State, StateCancelled)
	}
}

type testIdem struct {
	values map[string]Order
}

func newMemoryIdemForTest() *testIdem {
	return &testIdem{values: make(map[string]Order)}
}

func (s *testIdem) Get(_ context.Context, key string) (*Order, bool) {
	value, ok := s.values[key]
	return &value, ok
}

func (s *testIdem) Set(_ context.Context, key string, response *Order, _ time.Duration) error {
	s.values[key] = *response
	return nil
}
