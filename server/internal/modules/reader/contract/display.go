package contract

import "context"

type AccountDisplay struct {
	ID       int64
	Username string
	Nickname string
	Status   string
	Found    bool
}

// BatchAccountDisplays returns exactly one result for each input ID, in the
// same order. Missing accounts are represented by Found=false.
type AccountDisplayReader interface {
	BatchAccountDisplays(context.Context, []int64) ([]AccountDisplay, error)
}
