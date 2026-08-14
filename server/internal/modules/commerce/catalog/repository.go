package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Repository loads all facts required for one or many access decisions. A
// batch method is deliberately part of the public contract to prevent N+1
// chapter queries in the reader catalogue.
type Repository interface {
	LoadAccessContext(context.Context, AccessRequest) (AccessContext, error)
}

type BatchRepository interface {
	LoadAccessContexts(context.Context, []AccessRequest) (map[string]AccessContext, error)
}

type SQLRepository struct{ DB *sql.DB }

func (r SQLRepository) LoadAccessContext(ctx context.Context, req AccessRequest) (AccessContext, error) {
	var out AccessContext
	var product Product
	var target sql.NullInt64
	var found bool
	err := r.DB.QueryRowContext(ctx, `SELECT id,product_type,target_id,product_name,price_coin,sale_status FROM commerce_products WHERE product_type='book' AND target_id=$1`, req.BookID).
		Scan(&product.ID, &product.ProductType, &target, &product.ProductName, &product.PriceCoin, &product.SaleStatus)
	if err != nil && err != sql.ErrNoRows {
		return out, err
	}
	if err == nil {
		found = true
		if target.Valid {
			product.TargetID = &target.Int64
		}
		out.Product = &product
	}
	if req.ChapterID != 0 {
		var cp Product
		var ct sql.NullInt64
		err = r.DB.QueryRowContext(ctx, `SELECT id,product_type,target_id,product_name,price_coin,sale_status FROM commerce_products WHERE product_type='chapter' AND target_id=$1`, req.ChapterID).
			Scan(&cp.ID, &cp.ProductType, &ct, &cp.ProductName, &cp.PriceCoin, &cp.SaleStatus)
		if err == nil {
			if ct.Valid {
				cp.TargetID = &ct.Int64
			}
			out.ChapterProduct = &cp
		} else if err != sql.ErrNoRows {
			return out, err
		}
	}
	var p ChapterPricing
	if err = r.DB.QueryRowContext(ctx, `SELECT word_unit,coin_unit,enabled FROM commerce_chapter_pricing_config WHERE id=1`).Scan(&p.WordUnit, &p.CoinUnit, &p.Enabled); err != nil && err != sql.ErrNoRows {
		return out, err
	}
	if !found && err == sql.ErrNoRows {
		return out, nil
	}
	if req.ReaderID != nil {
		rows, qerr := r.DB.QueryContext(ctx, `SELECT entitlement_type,target_id FROM commerce_entitlements WHERE reader_id=$1 AND status='active' AND starts_at<=now() AND (permanent OR expires_at>now())`, *req.ReaderID)
		if qerr != nil {
			return out, qerr
		}
		for rows.Next() {
			var typ string
			var targetID int64
			if qerr = rows.Scan(&typ, &targetID); qerr != nil {
				rows.Close()
				return out, qerr
			}
			if typ == "book" && targetID == req.BookID {
				out.Reader.BookOwned = true
			}
			if typ == "chapter" && targetID == req.ChapterID {
				out.Reader.ChapterOwned = true
			}
		}
		rows.Close()
		if qerr = r.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM commerce_membership_grants WHERE reader_id=$1 AND status='active' AND starts_at<=now() AND (permanent OR expires_at>now()))`, *req.ReaderID).Scan(&out.Reader.Membership); qerr != nil {
			return out, qerr
		}
	}
	return out, nil
}

func (r SQLRepository) LoadAccessContexts(ctx context.Context, reqs []AccessRequest) (map[string]AccessContext, error) {
	out := make(map[string]AccessContext, len(reqs))
	if len(reqs) == 0 {
		return out, nil
	}
	args := make([]any, len(reqs))
	marks := make([]string, len(reqs))
	for i, req := range reqs {
		args[i] = req.BookID
		marks[i] = fmt.Sprintf("$%d", i+1)
		out[key(req)] = AccessContext{Reader: ReaderContext{ReaderID: req.ReaderID}}
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT id,product_type,target_id,product_name,price_coin,sale_status FROM commerce_products WHERE product_type='book' AND target_id IN (`+strings.Join(marks, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p Product
		var target int64
		if err = rows.Scan(&p.ID, &p.ProductType, &target, &p.ProductName, &p.PriceCoin, &p.SaleStatus); err != nil {
			return nil, err
		}
		p.TargetID = &target
		for _, req := range reqs {
			if req.BookID == target {
				c := out[key(req)]
				c.Product = &p
				out[key(req)] = c
			}
		}
	}
	chapterArgs := make([]any, 0)
	chapterMarks := make([]string, 0)
	for _, req := range reqs {
		if req.ChapterID != 0 {
			chapterArgs = append(chapterArgs, req.ChapterID)
			chapterMarks = append(chapterMarks, fmt.Sprintf("$%d", len(chapterArgs)))
		}
	}
	if len(chapterArgs) > 0 {
		crows, e := r.DB.QueryContext(ctx, `SELECT id,product_type,target_id,product_name,price_coin,sale_status FROM commerce_products WHERE product_type='chapter' AND target_id IN (`+strings.Join(chapterMarks, ",")+`)`, chapterArgs...)
		if e != nil {
			return nil, e
		}
		for crows.Next() {
			var p Product
			var target int64
			if e = crows.Scan(&p.ID, &p.ProductType, &target, &p.ProductName, &p.PriceCoin, &p.SaleStatus); e != nil {
				crows.Close()
				return nil, e
			}
			p.TargetID = &target
			for _, req := range reqs {
				if req.ChapterID == target {
					c := out[key(req)]
					c.ChapterProduct = &p
					out[key(req)] = c
				}
			}
		}
		crows.Close()
	}
	// Shared pricing is a singleton and is loaded once for the whole batch.
	var pricing ChapterPricing
	if err = r.DB.QueryRowContext(ctx, `SELECT word_unit,coin_unit,enabled FROM commerce_chapter_pricing_config WHERE id=1`).Scan(&pricing.WordUnit, &pricing.CoinUnit, &pricing.Enabled); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	for k, c := range out {
		c.Pricing = pricing
		out[k] = c
	}
	// Load reader facts in set-based queries as well. The mapping back to
	// requests is in memory and never issues one query per chapter.
	readerArgs := make([]any, 0)
	readerMarks := make([]string, 0)
	readers := make(map[int64]struct{})
	for _, req := range reqs {
		if req.ReaderID != nil {
			if _, ok := readers[*req.ReaderID]; !ok {
				readers[*req.ReaderID] = struct{}{}
				readerArgs = append(readerArgs, *req.ReaderID)
				readerMarks = append(readerMarks, fmt.Sprintf("$%d", len(readerArgs)))
			}
		}
	}
	if len(readerArgs) > 0 {
		erows, e := r.DB.QueryContext(ctx, `SELECT reader_id,entitlement_type,target_id FROM commerce_entitlements WHERE reader_id IN (`+strings.Join(readerMarks, ",")+`) AND status='active' AND starts_at<=now() AND (permanent OR expires_at>now())`, readerArgs...)
		if e != nil {
			return nil, e
		}
		for erows.Next() {
			var rid, target int64
			var typ string
			if e = erows.Scan(&rid, &typ, &target); e != nil {
				erows.Close()
				return nil, e
			}
			for _, req := range reqs {
				if req.ReaderID != nil && *req.ReaderID == rid {
					c := out[key(req)]
					if typ == "book" && target == req.BookID {
						c.Reader.BookOwned = true
					}
					if typ == "chapter" && target == req.ChapterID {
						c.Reader.ChapterOwned = true
					}
					out[key(req)] = c
				}
			}
		}
		erows.Close()
		mrows, e := r.DB.QueryContext(ctx, `SELECT DISTINCT reader_id FROM commerce_membership_grants WHERE reader_id IN (`+strings.Join(readerMarks, ",")+`) AND status='active' AND starts_at<=now() AND (permanent OR expires_at>now())`, readerArgs...)
		if e != nil {
			return nil, e
		}
		for mrows.Next() {
			var rid int64
			if e = mrows.Scan(&rid); e != nil {
				mrows.Close()
				return nil, e
			}
			for _, req := range reqs {
				if req.ReaderID != nil && *req.ReaderID == rid {
					c := out[key(req)]
					c.Reader.Membership = true
					out[key(req)] = c
				}
			}
		}
		mrows.Close()
	}
	return out, nil
}

func key(r AccessRequest) string { return decimal(r.BookID) + ":" + decimal(r.ChapterID) }
