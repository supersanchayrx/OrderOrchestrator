package order

import (
	"time"
)

type Order struct {
	ID          string      `json:"id"`
	CustomerID  string      `json:"customerId"`
	State       State       `json:"state"`
	TotalAmount string      `json:"totalAmount"`
	Currency    string      `json:"currency"`
	Items       []OrderItem `json:"items"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	Version     int         `json:"version"`
}

type OrderItem struct {
	ID        string `json:"id"`
	OrderID   string `json:"orderId"`
	SKUID     string `json:"skuId"`
	Quantity  int    `json:"quantity"`
	UnitPrice string `json:"unitPrice"`
}

type Transition struct {
	ID          int64          `json:"id"`
	OrderID     string         `json:"orderId"`
	FromState   *State         `json:"fromState,omitempty"`
	ToState     State          `json:"toState"`
	TriggeredBy string         `json:"triggeredBy"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
}

type Timeout struct {
	ID            int64     `json:"id"`
	OrderID       string    `json:"orderId"`
	ExpectedState State     `json:"expectedState"`
	Deadline      time.Time `json:"deadline"`
	Processed     bool      `json:"processed"`
}

type CreateOrderRequest struct {
	CustomerID  string            `json:"customerId"`
	TotalAmount string            `json:"totalAmount"`
	Currency    string            `json:"currency"`
	Items       []CreateOrderItem `json:"items"`
}

type CreateOrderItem struct {
	SKUID     string `json:"skuId"`
	Quantity  int    `json:"quantity"`
	UnitPrice string `json:"unitPrice"`
}

type TransitionRequest struct {
	TargetState State          `json:"targetState"`
	TriggeredBy string         `json:"triggeredBy"`
	Metadata    map[string]any `json:"metadata"`
}

type OrderStateChangedEvent struct {
	OrderID   string    `json:"orderId"`
	From      State     `json:"from"`
	To        State     `json:"to"`
	Timestamp time.Time `json:"timestamp"`
}
