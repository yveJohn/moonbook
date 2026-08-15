//go:build integration

package provider

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

func TestInviteProviderReadsActiveRelationInsideTransaction(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	inviterID, inviteeID, cancelledInviteeID, codeID, cancelledCodeID := base, base+1, base+2, base+3, base+4
	suffix := fmt.Sprintf("invite-provider-%d", base)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,$2,'x'),($3,$4,'x'),($5,$6,'x')`, inviterID, suffix+"-inviter", inviteeID, suffix+"-invitee", cancelledInviteeID, suffix+"-cancelled"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status) VALUES($1,$2,$3,'enabled'),($4,$5,NULL,'enabled')`, codeID, suffix, inviterID, cancelledCodeID, suffix+"-cancelled"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_relations(inviter_reader_id,invitee_reader_id,invite_code_id,status) VALUES($1,$2,$3,'active'),($1,$4,$5,'cancelled')`, inviterID, inviteeID, codeID, cancelledInviteeID, cancelledCodeID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		q := context.Background()
		_, _ = db.ExecContext(q, `DELETE FROM reader_invite_relations WHERE invitee_reader_id IN ($1,$2)`, inviteeID, cancelledInviteeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_invite_codes WHERE id IN ($1,$2)`, codeID, cancelledCodeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_accounts WHERE id IN ($1,$2,$3)`, inviterID, inviteeID, cancelledInviteeID)
	})
	provider := NewInvite(db)
	if _, err := provider.ActiveInviteRelation(ctx, inviteeID); !errors.Is(err, readercontract.ErrUnavailable) || !errors.Is(readercontract.Cause(err), transaction.ErrNoTransaction) {
		t.Fatalf("outside transaction err=%v", err)
	}
	if err := transaction.New(db).Within(ctx, func(txCtx context.Context) error {
		relation, err := provider.ActiveInviteRelation(txCtx, inviteeID)
		if err != nil || relation.InviterID != inviterID || relation.InviteeID != inviteeID || relation.Status != "active" {
			t.Fatalf("relation=%+v err=%v", relation, err)
		}
		if _, err := provider.ActiveInviteRelation(txCtx, cancelledInviteeID); !errors.Is(err, readercontract.ErrInviteRelationNotFound) {
			t.Fatalf("cancelled relation err=%v", err)
		}
		if _, err := provider.ActiveInviteRelation(txCtx, base+999); !errors.Is(err, readercontract.ErrInviteRelationNotFound) {
			t.Fatalf("missing relation err=%v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
