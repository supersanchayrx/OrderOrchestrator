package order

import "fmt"

type State string

const (
	StatePlaced         State = "PLACED"
	StatePaymentPending State = "PAYMENT_PENDING"
	StateConfirmed      State = "CONFIRMED"
	StateProcessing     State = "PROCESSING"
	StatePacked         State = "PACKED"
	StateDispatched     State = "DISPATCHED"
	StateOutForDelivery State = "OUT_FOR_DELIVERY"
	StateDelivered      State = "DELIVERED"
	StateCancelled      State = "CANCELLED"
	StatePaymentFailed  State = "PAYMENT_FAILED"
	StateFailedDelivery State = "FAILED_DELIVERY"
	StateRefundIssued   State = "REFUND_ISSUED"
)

var allowedTransitions = map[State][]State{
	StatePlaced:         {StatePaymentPending, StateCancelled},
	StatePaymentPending: {StateConfirmed, StatePaymentFailed},
	StateConfirmed:      {StateProcessing, StateCancelled},
	StateProcessing:     {StatePacked},
	StatePacked:         {StateDispatched},
	StateDispatched:     {StateOutForDelivery},
	StateOutForDelivery: {StateDelivered, StateFailedDelivery},
	StateFailedDelivery: {StateOutForDelivery, StateRefundIssued},
}

func CanTransition(from, to State) bool {
	for _, candidate := range allowedTransitions[from] {
		if candidate == to {
			return true
		}
	}
	return false
}

func IsTerminal(state State) bool {
	switch state {
	case StateDelivered, StateCancelled, StatePaymentFailed, StateRefundIssued:
		return true
	default:
		return false
	}
}

type ErrInvalidTransition struct {
	From State
	To   State
}

func (e *ErrInvalidTransition) Error() string {
	return fmt.Sprintf("invalid transition: %s -> %s", e.From, e.To)
}
