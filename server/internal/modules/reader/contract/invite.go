package contract

import "context"

type InviteRelation struct {
	ID        int64
	InviterID int64
	InviteeID int64
	Status    string
}

type InviteRelationReader interface {
	ActiveInviteRelation(context.Context, int64) (InviteRelation, error)
}
