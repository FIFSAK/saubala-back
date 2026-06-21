package shipment

import "fmt"

type Status string

const (
	StatusPending   Status = "pending"
	StatusPickedUp  Status = "picked_up"
	StatusInTransit Status = "in_transit"
	StatusDelivered Status = "delivered"
	StatusCancelled Status = "cancelled"
)

var validTransitions = map[Status][]Status{
	StatusPending:   {StatusPickedUp, StatusCancelled},
	StatusPickedUp:  {StatusInTransit, StatusCancelled},
	StatusInTransit: {StatusDelivered, StatusCancelled},
	StatusDelivered: {},
	StatusCancelled: {},
}

var allStatuses = map[Status]bool{
	StatusPending:   true,
	StatusPickedUp:  true,
	StatusInTransit: true,
	StatusDelivered: true,
	StatusCancelled: true,
}

func (s Status) IsValid() bool {
	return allStatuses[s]
}

func (s Status) CanTransitionTo(next Status) bool {
	allowed, ok := validTransitions[s]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == next {
			return true
		}
	}
	return false
}

func (s Status) ValidateTransition(next Status) error {
	if !next.IsValid() {
		return fmt.Errorf("invalid status: %s", next)
	}
	if !s.CanTransitionTo(next) {
		return fmt.Errorf("invalid transition from %s to %s", s, next)
	}
	return nil
}
