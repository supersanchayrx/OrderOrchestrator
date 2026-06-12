package order

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"
)

var (
	ErrNotFound               = errors.New("order not found")
	ErrConcurrentModification = errors.New("order was modified concurrently")
)

type Repository interface {
	Create(ctx context.Context, order Order, initial Transition, timeout *Timeout) (Order, error)
	Get(ctx context.Context, orderID string) (Order, error)
	GetForUpdate(ctx context.Context, orderID string) (Order, error)
	UpdateState(ctx context.Context, orderID string, state State, expectedVersion int) (Order, error)
	InsertTransition(ctx context.Context, transition Transition) (Transition, error)
	ListTransitions(ctx context.Context, orderID string) ([]Transition, error)
	InsertTimeout(ctx context.Context, timeout Timeout) (Timeout, error)
	FindDueUnprocessedTimeouts(ctx context.Context, now time.Time) ([]Timeout, error)
	MarkTimeoutProcessed(ctx context.Context, timeoutID int64) error
}

type MemoryRepository struct {
	mu             sync.Mutex
	orders         map[string]Order
	transitions    map[string][]Transition
	timeouts       map[int64]Timeout
	nextTransition int64
	nextTimeout    int64
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		orders:      make(map[string]Order),
		transitions: make(map[string][]Transition),
		timeouts:    make(map[int64]Timeout),
	}
}

func (r *MemoryRepository) Create(_ context.Context, order Order, initial Transition, timeout *Timeout) (Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.orders[order.ID] = order
	r.nextTransition++
	initial.ID = r.nextTransition
	r.transitions[order.ID] = append(r.transitions[order.ID], initial)

	if timeout != nil {
		r.nextTimeout++
		timeout.ID = r.nextTimeout
		r.timeouts[timeout.ID] = *timeout
	}

	return cloneOrder(order), nil
}

func (r *MemoryRepository) Get(ctx context.Context, orderID string) (Order, error) {
	return r.GetForUpdate(ctx, orderID)
}

func (r *MemoryRepository) GetForUpdate(_ context.Context, orderID string) (Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	o, ok := r.orders[orderID]
	if !ok {
		return Order{}, ErrNotFound
	}
	return cloneOrder(o), nil
}

func (r *MemoryRepository) UpdateState(_ context.Context, orderID string, state State, expectedVersion int) (Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	o, ok := r.orders[orderID]
	if !ok {
		return Order{}, ErrNotFound
	}
	if o.Version != expectedVersion {
		return Order{}, ErrConcurrentModification
	}
	o.State = state
	o.Version++
	o.UpdatedAt = time.Now().UTC()
	r.orders[orderID] = o

	return cloneOrder(o), nil
}

func (r *MemoryRepository) InsertTransition(_ context.Context, transition Transition) (Transition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.orders[transition.OrderID]; !ok {
		return Transition{}, ErrNotFound
	}

	r.nextTransition++
	transition.ID = r.nextTransition
	r.transitions[transition.OrderID] = append(r.transitions[transition.OrderID], cloneTransition(transition))
	return cloneTransition(transition), nil
}

func (r *MemoryRepository) ListTransitions(_ context.Context, orderID string) ([]Transition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.orders[orderID]; !ok {
		return nil, ErrNotFound
	}
	history := slices.Clone(r.transitions[orderID])
	for i := range history {
		history[i] = cloneTransition(history[i])
	}
	return history, nil
}

func (r *MemoryRepository) InsertTimeout(_ context.Context, timeout Timeout) (Timeout, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextTimeout++
	timeout.ID = r.nextTimeout
	r.timeouts[timeout.ID] = timeout
	return timeout, nil
}

func (r *MemoryRepository) FindDueUnprocessedTimeouts(_ context.Context, now time.Time) ([]Timeout, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	due := make([]Timeout, 0)
	for _, timeout := range r.timeouts {
		if !timeout.Processed && !timeout.Deadline.After(now) {
			due = append(due, timeout)
		}
	}
	return due, nil
}

func (r *MemoryRepository) MarkTimeoutProcessed(_ context.Context, timeoutID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	timeout, ok := r.timeouts[timeoutID]
	if !ok {
		return ErrNotFound
	}
	timeout.Processed = true
	r.timeouts[timeoutID] = timeout
	return nil
}

func cloneOrder(order Order) Order {
	order.Items = slices.Clone(order.Items)
	return order
}

func cloneTransition(transition Transition) Transition {
	if transition.Metadata != nil {
		metadata := make(map[string]any, len(transition.Metadata))
		for key, value := range transition.Metadata {
			metadata[key] = value
		}
		transition.Metadata = metadata
	}
	return transition
}
