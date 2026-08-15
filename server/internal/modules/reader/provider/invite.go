package provider

import (
	"context"
	"database/sql"
	"errors"

	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

type Invite struct{ db *sql.DB }

var _ readercontract.InviteRelationReader = (*Invite)(nil)

func NewInvite(db *sql.DB) *Invite { return &Invite{db: db} }

func (provider *Invite) ActiveInviteRelation(ctx context.Context, inviteeID int64) (readercontract.InviteRelation, error) {
	if provider == nil || provider.db == nil || inviteeID <= 0 {
		return readercontract.InviteRelation{}, readercontract.ErrInviteRelationNotFound
	}
	executor := transaction.Executor(ctx, provider.db)
	if executor == provider.db {
		return readercontract.InviteRelation{}, readercontract.Wrap(readercontract.ErrUnavailable, transaction.ErrNoTransaction)
	}
	var relation readercontract.InviteRelation
	err := executor.QueryRowContext(ctx, `SELECT id,inviter_reader_id,invitee_reader_id,status FROM reader_invite_relations WHERE invitee_reader_id=$1 FOR SHARE`, inviteeID).
		Scan(&relation.ID, &relation.InviterID, &relation.InviteeID, &relation.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return readercontract.InviteRelation{}, readercontract.ErrInviteRelationNotFound
	}
	if err != nil {
		return readercontract.InviteRelation{}, readercontract.Wrap(readercontract.ErrUnavailable, err)
	}
	if relation.Status != "active" {
		return readercontract.InviteRelation{}, readercontract.ErrInviteRelationNotFound
	}
	return relation, nil
}
