package order

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type EventPublisher interface {
	Publish(ctx context.Context, event OrderStateChangedEvent) error
}

type IdempotencyStore interface {
	Get(ctx context.Context, key string) (*Order, bool)
	Set(ctx context.Context, key string, response *Order, ttl time.Duration) error
}

type Broadcaster interface {
	Broadcast(orderID string, payload any)
}

type Service struct {
	repo      Repository
	publisher EventPublisher
	idem      IdempotencyStore
	hub       Broadcaster
	logger    *slog.Logger
}

func NewService(repo Repository, publisher EventPublisher, idem IdempotencyStore, hub Broadcaster, logger *slog.Logger) *Service {
	return &Service{repo: repo, publisher: publisher, idem: idem, hub: hub, logger: logger}
}

func (s *Service) Create(ctx context.Context, req CreateOrderRequest) (Order, error) {
	now := time.Now().UTC()
	orderID := newID("ord")
	o := Order{
		ID:          orderID,
		CustomerID:  req.CustomerID,
		State:       StatePlaced,
		TotalAmount: req.TotalAmount,
		Currency:    defaultCurrency(req.Currency),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	for _, item := range req.Items {
		o.Items = append(o.Items, OrderItem{
			ID:        newID("item"),
			OrderID:   orderID,
			SKUID:     item.SKUID,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		})
	}

	initial := Transition{
		OrderID:     orderID,
		ToState:     StatePlaced,
		TriggeredBy: "USER",
		CreatedAt:   now,
	}

	timeout := &Timeout{
		OrderID:       orderID,
		ExpectedState: StatePlaced,
		Deadline:      now.Add(10 * time.Minute),
	}

	created, err := s.repo.Create(ctx, o, initial, timeout)
	if err != nil {
		return Order{}, err
	}
	s.broadcast(created)
	return created, nil
}

func (s *Service) Get(ctx context.Context, orderID string) (Order, error) {
	return s.repo.Get(ctx, orderID)
}

func (s *Service) History(ctx context.Context, orderID string) ([]Transition, error) {
	return s.repo.ListTransitions(ctx, orderID)
}

func (s *Service) Transition(ctx context.Context, orderID string, req TransitionRequest, idemKey string) (Order, error) {
	if idemKey != "" {
		if cached, ok := s.idem.Get(ctx, idemKey); ok {
			return *cached, nil
		}
	}

	o, err := s.repo.GetForUpdate(ctx, orderID)
	if err != nil {
		return Order{}, err
	}
	if !CanTransition(o.State, req.TargetState) {
		return Order{}, &ErrInvalidTransition{From: o.State, To: req.TargetState}
	}

	previous := o.State
	updated, err := s.repo.UpdateState(ctx, orderID, req.TargetState, o.Version)
	if err != nil {
		return Order{}, err
	}

	transition := Transition{
		OrderID:     orderID,
		FromState:   &previous,
		ToState:     req.TargetState,
		TriggeredBy: defaultTrigger(req.TriggeredBy),
		Metadata:    req.Metadata,
		CreatedAt:   updated.UpdatedAt,
	}
	if _, err := s.repo.InsertTransition(ctx, transition); err != nil {
		return Order{}, err
	}

	if err := s.scheduleTimeout(ctx, updated); err != nil {
		s.logger.Warn("failed to schedule timeout", "orderId", orderID, "state", updated.State, "error", err)
	}

	event := OrderStateChangedEvent{
		OrderID:   orderID,
		From:      previous,
		To:        updated.State,
		Timestamp: updated.UpdatedAt,
	}
	if err := s.publisher.Publish(ctx, event); err != nil {
		s.logger.Warn("event publish failed", "orderId", orderID, "error", err)
	}
	s.broadcast(updated)

	if idemKey != "" {
		if err := s.idem.Set(ctx, idemKey, &updated, 24*time.Hour); err != nil {
			s.logger.Warn("idempotency cache write failed", "key", idemKey, "error", err)
		}
	}

	return updated, nil
}

func (s *Service) Cancel(ctx context.Context, orderID string, idemKey string) (Order, error) {
	return s.Transition(ctx, orderID, TransitionRequest{
		TargetState: StateCancelled,
		TriggeredBy: "USER",
	}, idemKey)
}

func (s *Service) TimeoutTransition(ctx context.Context, orderID string, expected State) (Order, error) {
	target, ok := timeoutTargetState(expected)
	if !ok {
		return Order{}, fmt.Errorf("no timeout transition for state %s", expected)
	}

	current, err := s.Get(ctx, orderID)
	if err != nil {
		return Order{}, err
	}
	if current.State != expected || IsTerminal(current.State) {
		return current, nil
	}

	return s.Transition(ctx, orderID, TransitionRequest{
		TargetState: target,
		TriggeredBy: "SYSTEM_TIMEOUT",
	}, newID("timeout"))
}

func (s *Service) scheduleTimeout(ctx context.Context, o Order) error {
	var deadline time.Time
	switch o.State {
	case StatePlaced:
		deadline = o.UpdatedAt.Add(10 * time.Minute)
	case StatePaymentPending:
		deadline = o.UpdatedAt.Add(15 * time.Minute)
	default:
		return nil
	}

	_, err := s.repo.InsertTimeout(ctx, Timeout{
		OrderID:       o.ID,
		ExpectedState: o.State,
		Deadline:      deadline,
	})
	return err
}

func (s *Service) broadcast(o Order) {
	if s.hub != nil {
		s.hub.Broadcast(o.ID, o)
	}
}

func timeoutTargetState(state State) (State, bool) {
	switch state {
	case StatePlaced:
		return StateCancelled, true
	case StatePaymentPending:
		return StatePaymentFailed, true
	default:
		return "", false
	}
}

func defaultCurrency(currency string) string {
	if currency == "" {
		return "INR"
	}
	return currency
}

func defaultTrigger(trigger string) string {
	if trigger == "" {
		return "SYSTEM"
	}
	return trigger
}

func newID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(errors.New("crypto/rand failed"))
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
