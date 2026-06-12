package order

import "testing"

func TestCanTransition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		from State
		to   State
		want bool
	}{
		{name: "placed to payment pending", from: StatePlaced, to: StatePaymentPending, want: true},
		{name: "placed to cancelled", from: StatePlaced, to: StateCancelled, want: true},
		{name: "payment pending to confirmed", from: StatePaymentPending, to: StateConfirmed, want: true},
		{name: "confirmed to processing", from: StateConfirmed, to: StateProcessing, want: true},
		{name: "delivered cannot move", from: StateDelivered, to: StateRefundIssued, want: false},
		{name: "cannot skip to delivered", from: StatePlaced, to: StateDelivered, want: false},
		{name: "cannot cancel after dispatch", from: StateDispatched, to: StateCancelled, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := CanTransition(tt.from, tt.to); got != tt.want {
				t.Fatalf("CanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	t.Parallel()

	for _, state := range []State{StateDelivered, StateCancelled, StatePaymentFailed, StateRefundIssued} {
		if !IsTerminal(state) {
			t.Fatalf("%s should be terminal", state)
		}
	}

	for _, state := range []State{StatePlaced, StateProcessing, StateFailedDelivery} {
		if IsTerminal(state) {
			t.Fatalf("%s should not be terminal", state)
		}
	}
}
