package objectstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	extensionPattern   = regexp.MustCompile(`^[a-z0-9]{1,10}$`)
	fingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Service struct {
	db    *sql.DB
	blobs BlobStore
}

func NewService(db *sql.DB, blobs BlobStore) *Service {
	return &Service{db: db, blobs: blobs}
}

func normalizeTarget(target Target) (Target, error) {
	if target.BookID <= 0 || target.OwnerID <= 0 {
		return Target{}, errors.New("book and owner IDs must be positive")
	}
	switch target.Kind {
	case KindChapterContent:
		target.Extension = "txt"
	case KindBookCover:
		if target.OwnerID != target.BookID {
			return Target{}, errors.New("cover owner ID must equal book ID")
		}
		target.Extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(target.Extension)), ".")
		if !extensionPattern.MatchString(target.Extension) {
			return Target{}, errors.New("cover extension is invalid")
		}
	default:
		return Target{}, errors.New("object kind is invalid")
	}
	return target, nil
}

func objectKey(target Target, version int) string {
	if target.Kind == KindChapterContent {
		return fmt.Sprintf("chapters/%d/%d/v%d.txt", target.BookID, target.OwnerID, version)
	}
	return fmt.Sprintf("covers/%d/v%d.%s", target.BookID, version, target.Extension)
}

func (service *Service) UploadVerified(ctx context.Context, target Target, content []byte, contentType string) (Object, error) {
	return service.UploadVerifiedWithOptions(ctx, target, content, contentType, UploadOptions{Source: "native"})
}

func (service *Service) UploadVerifiedWithOptions(ctx context.Context, target Target, content []byte, contentType string, options UploadOptions) (Object, error) {
	clean, err := normalizeTarget(target)
	if err != nil {
		return Object{}, err
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" || len(contentType) > 191 {
		return Object{}, errors.New("content type is required")
	}
	options.Source = strings.TrimSpace(options.Source)
	options.SourceFingerprint = strings.TrimSpace(options.SourceFingerprint)
	if options.Source == "" {
		options.Source = "native"
	}
	if options.Source != "native" && options.Source != "legacy" && options.Source != "import" {
		return Object{}, errors.New("object source is invalid")
	}
	if options.SourceFingerprint != "" && !fingerprintPattern.MatchString(options.SourceFingerprint) {
		return Object{}, errors.New("source fingerprint must be a lowercase SHA-256")
	}
	if options.Source == "legacy" && options.SourceFingerprint == "" {
		return Object{}, errors.New("legacy objects require a source fingerprint")
	}
	if options.SourceFingerprint != "" {
		connection, err := service.db.Conn(ctx)
		if err != nil {
			return Object{}, fmt.Errorf("open source object lock connection: %w", err)
		}
		defer connection.Close()
		lockKey := fmt.Sprintf("source:%s:%d:%d:%s", clean.Kind, clean.BookID, clean.OwnerID, options.SourceFingerprint)
		if _, err := connection.ExecContext(ctx, `SELECT pg_advisory_lock(hashtextextended($1, 1))`, lockKey); err != nil {
			return Object{}, fmt.Errorf("lock source object: %w", err)
		}
		defer func() {
			unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = connection.ExecContext(unlockCtx, `SELECT pg_advisory_unlock(hashtextextended($1, 1))`, lockKey)
		}()
	}
	digest := sha256.Sum256(content)
	hash := hex.EncodeToString(digest[:])
	if options.SourceFingerprint != "" {
		if existing, found, err := service.findSourceObject(ctx, clean, options.SourceFingerprint); err != nil {
			return Object{}, err
		} else if found {
			return existing, nil
		}
	}
	object, err := service.reserve(ctx, clean, contentType, options)
	if err != nil {
		return Object{}, err
	}
	fail := func(code string, cause error) (Object, error) {
		if markErr := service.markFailed(ctx, object.ID, code); markErr != nil {
			return Object{}, errors.Join(cause, markErr)
		}
		return Object{}, cause
	}
	if err := service.blobs.Put(ctx, object.Key, bytes.NewReader(content), int64(len(content)), contentType, hash); err != nil {
		return fail("PUT_FAILED", err)
	}
	stat, err := service.blobs.Stat(ctx, object.Key)
	if err != nil {
		return fail("STAT_FAILED", err)
	}
	if stat.Key != object.Key || stat.ByteSize != int64(len(content)) || !strings.EqualFold(stat.SHA256, hash) {
		return fail("VERIFY_MISMATCH", fmt.Errorf("uploaded object verification mismatch"))
	}
	if err := service.markVerified(ctx, object.ID, hash, int64(len(content))); err != nil {
		return Object{}, err
	}
	object.SHA256 = hash
	object.ByteSize = int64(len(content))
	object.State = StateVerified
	return object, nil
}

func (service *Service) reserve(ctx context.Context, target Target, contentType string, options UploadOptions) (Object, error) {
	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return Object{}, err
	}
	defer tx.Rollback()
	lockKey := fmt.Sprintf("%s:%d:%d", target.Kind, target.BookID, target.OwnerID)
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return Object{}, fmt.Errorf("lock object target: %w", err)
	}
	var version int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(max(version), 0) + 1 FROM novel_objects
		WHERE object_kind=$1 AND book_id=$2 AND owner_id=$3`, target.Kind, target.BookID, target.OwnerID).Scan(&version); err != nil {
		return Object{}, fmt.Errorf("allocate object version: %w", err)
	}
	key := objectKey(target, version)
	var object Object
	err = tx.QueryRowContext(ctx, `INSERT INTO novel_objects
		(object_kind,book_id,owner_id,version,object_key,content_type,source,source_fingerprint)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''))
		RETURNING id,created_at`, target.Kind, target.BookID, target.OwnerID, version, key, contentType, options.Source, options.SourceFingerprint).
		Scan(&object.ID, &object.CreatedAt)
	if err != nil {
		return Object{}, fmt.Errorf("reserve object version: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO novel_object_events (object_id,event_type) VALUES ($1,'reserved')`, object.ID); err != nil {
		return Object{}, fmt.Errorf("record object reservation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Object{}, fmt.Errorf("commit object reservation: %w", err)
	}
	object.Target = target
	object.Version = version
	object.Key = key
	object.ContentType = contentType
	object.State = StateUploading
	return object, nil
}

func (service *Service) findSourceObject(ctx context.Context, target Target, fingerprint string) (Object, bool, error) {
	var object Object
	err := service.db.QueryRowContext(ctx, `SELECT id,version,object_key,sha256,byte_size,content_type,state,created_at
		FROM novel_objects WHERE object_kind=$1 AND book_id=$2 AND owner_id=$3 AND source_fingerprint=$4
		AND (state IN ('verified','active') OR (state='orphaned' AND error_code IS DISTINCT FROM 'ACTIVATION_FAILED'))`, target.Kind, target.BookID, target.OwnerID, fingerprint).
		Scan(&object.ID, &object.Version, &object.Key, &object.SHA256, &object.ByteSize, &object.ContentType, &object.State, &object.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Object{}, false, nil
	}
	if err != nil {
		return Object{}, false, fmt.Errorf("load object by source fingerprint: %w", err)
	}
	object.Target = target
	return object, true, nil
}

func (service *Service) markVerified(ctx context.Context, id int64, hash string, size int64) error {
	result, err := service.db.ExecContext(ctx, `WITH changed AS (
		UPDATE novel_objects SET state='verified',sha256=$2,byte_size=$3,verified_at=now()
		WHERE id=$1 AND state='uploading' RETURNING id)
		INSERT INTO novel_object_events (object_id,event_type)
		SELECT id,'verified' FROM changed`, id, hash, size)
	if err != nil {
		return fmt.Errorf("mark object verified: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return errors.New("object is no longer awaiting verification")
	}
	return nil
}

func (service *Service) markFailed(ctx context.Context, id int64, code string) error {
	_, err := service.db.ExecContext(ctx, `WITH changed AS (
		UPDATE novel_objects SET state='failed',error_code=$2 WHERE id=$1 AND state='uploading' RETURNING id)
		INSERT INTO novel_object_events (object_id,event_type,detail)
		SELECT id,'failed',jsonb_build_object('errorCode',$2::text) FROM changed`, id, code)
	if err != nil {
		return fmt.Errorf("mark object failed: %w", err)
	}
	return nil
}

func (service *Service) Activate(ctx context.Context, id int64) error {
	return service.ActivateWithTx(ctx, id, nil)
}

// ActivateWithTx switches the object reference and applies the owning business
// change in the same PostgreSQL transaction.
func (service *Service) ActivateWithTx(ctx context.Context, id int64, apply func(*sql.Tx, Target) error) error {
	err := service.activateWithTx(ctx, id, apply)
	if err == nil {
		return nil
	}
	if abandonErr := service.abandonVerified(ctx, id); abandonErr != nil {
		return errors.Join(err, abandonErr)
	}
	return err
}

// ActivateManyWithTx activates multiple verified objects and applies one
// owning business change atomically. It is used by workflows that create a
// complete set of related objects, such as a merged book and its chapters.
func (service *Service) ActivateManyWithTx(ctx context.Context, ids []int64, apply func(*sql.Tx, []Target) error) error {
	if len(ids) == 0 {
		return errors.New("at least one object is required")
	}
	ordered := append([]int64(nil), ids...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for index, id := range ordered {
		if id <= 0 || (index > 0 && id == ordered[index-1]) {
			return errors.New("object IDs must be unique and positive")
		}
	}
	_, err := service.activateManyWithTx(ctx, ordered, apply)
	if err == nil {
		return nil
	}
	for _, id := range ordered {
		if abandonErr := service.abandonVerified(ctx, id); abandonErr != nil {
			err = errors.Join(err, abandonErr)
		}
	}
	return err
}

// AbandonVerified marks verified, unreferenced objects as orphaned so the GC
// worker can remove uploads that a multi-step business operation cannot use.
func (service *Service) AbandonVerified(ctx context.Context, ids []int64) error {
	var result error
	for _, id := range ids {
		if id <= 0 {
			result = errors.Join(result, errors.New("object ID must be positive"))
			continue
		}
		if err := service.abandonVerified(ctx, id); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func (service *Service) activateManyWithTx(ctx context.Context, ids []int64, apply func(*sql.Tx, []Target) error) ([]Target, error) {
	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	targets := make([]Target, 0, len(ids))
	seenTargets := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		var target Target
		var state string
		if err := tx.QueryRowContext(ctx, `SELECT object_kind,book_id,owner_id,state FROM novel_objects WHERE id=$1 FOR UPDATE`, id).
			Scan(&target.Kind, &target.BookID, &target.OwnerID, &state); err != nil {
			return targets, fmt.Errorf("load object for batch activation: %w", err)
		}
		if state != StateVerified {
			return targets, errors.New("only verified objects can be batch activated")
		}
		key := fmt.Sprintf("%s:%d:%d", target.Kind, target.BookID, target.OwnerID)
		if _, exists := seenTargets[key]; exists {
			return targets, errors.New("batch activation contains duplicate targets")
		}
		seenTargets[key] = struct{}{}
		targets = append(targets, target)
	}
	for index, target := range targets {
		lockKey := fmt.Sprintf("%s:%d:%d", target.Kind, target.BookID, target.OwnerID)
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
			return targets, fmt.Errorf("lock batch object target: %w", err)
		}
		var oldID sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT object_id FROM novel_object_references
			WHERE object_kind=$1 AND book_id=$2 AND owner_id=$3 FOR UPDATE`, target.Kind, target.BookID, target.OwnerID).Scan(&oldID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return targets, fmt.Errorf("lock batch object reference: %w", err)
		}
		id := ids[index]
		if _, err := tx.ExecContext(ctx, `INSERT INTO novel_object_references (object_kind,book_id,owner_id,object_id)
			VALUES ($1,$2,$3,$4) ON CONFLICT (object_kind,book_id,owner_id)
			DO UPDATE SET object_id=EXCLUDED.object_id,updated_at=now()`, target.Kind, target.BookID, target.OwnerID, id); err != nil {
			return targets, fmt.Errorf("switch batch object reference: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE novel_objects SET state='active',activated_at=now() WHERE id=$1`, id); err != nil {
			return targets, fmt.Errorf("activate batch object: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO novel_object_events (object_id,event_type) VALUES ($1,'activated')`, id); err != nil {
			return targets, fmt.Errorf("record batch object activation: %w", err)
		}
		if oldID.Valid && oldID.Int64 != id {
			if _, err := tx.ExecContext(ctx, `UPDATE novel_objects SET state='orphaned',orphaned_at=now()
				WHERE id=$1 AND state='active'`, oldID.Int64); err != nil {
				return targets, fmt.Errorf("orphan previous batch object: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO novel_object_events (object_id,event_type) VALUES ($1,'orphaned')`, oldID.Int64); err != nil {
				return targets, fmt.Errorf("record previous batch object orphan: %w", err)
			}
		}
	}
	if apply != nil {
		if err := apply(tx, targets); err != nil {
			return targets, fmt.Errorf("apply batch object activation business change: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return targets, fmt.Errorf("commit batch object activation: %w", err)
	}
	return targets, nil
}

func (service *Service) activateWithTx(ctx context.Context, id int64, apply func(*sql.Tx, Target) error) error {
	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var target Target
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT object_kind,book_id,owner_id,state FROM novel_objects WHERE id=$1 FOR UPDATE`, id).
		Scan(&target.Kind, &target.BookID, &target.OwnerID, &state); err != nil {
		return fmt.Errorf("load object for activation: %w", err)
	}
	if state != StateVerified {
		return fmt.Errorf("only verified objects can be activated")
	}
	lockKey := fmt.Sprintf("%s:%d:%d", target.Kind, target.BookID, target.OwnerID)
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return fmt.Errorf("lock object target for activation: %w", err)
	}
	var oldID sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT object_id FROM novel_object_references
		WHERE object_kind=$1 AND book_id=$2 AND owner_id=$3 FOR UPDATE`, target.Kind, target.BookID, target.OwnerID).Scan(&oldID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("lock current object reference: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO novel_object_references (object_kind,book_id,owner_id,object_id)
		VALUES ($1,$2,$3,$4) ON CONFLICT (object_kind,book_id,owner_id)
		DO UPDATE SET object_id=EXCLUDED.object_id,updated_at=now()`, target.Kind, target.BookID, target.OwnerID, id); err != nil {
		return fmt.Errorf("switch object reference: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE novel_objects SET state='active',activated_at=now() WHERE id=$1`, id); err != nil {
		return fmt.Errorf("activate object: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO novel_object_events (object_id,event_type) VALUES ($1,'activated')`, id); err != nil {
		return fmt.Errorf("record object activation: %w", err)
	}
	if oldID.Valid && oldID.Int64 != id {
		if _, err := tx.ExecContext(ctx, `UPDATE novel_objects SET state='orphaned',orphaned_at=now()
			WHERE id=$1 AND state='active'`, oldID.Int64); err != nil {
			return fmt.Errorf("orphan previous object: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO novel_object_events (object_id,event_type) VALUES ($1,'orphaned')`, oldID.Int64); err != nil {
			return fmt.Errorf("record previous object orphan: %w", err)
		}
	}
	if apply != nil {
		if err := apply(tx, target); err != nil {
			return fmt.Errorf("apply object activation business change: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit object activation: %w", err)
	}
	return nil
}

func (service *Service) abandonVerified(ctx context.Context, id int64) error {
	_, err := service.db.ExecContext(ctx, `WITH changed AS (
		UPDATE novel_objects o SET state='orphaned',orphaned_at=now(),error_code='ACTIVATION_FAILED'
		WHERE o.id=$1 AND o.state='verified'
		AND NOT EXISTS (SELECT 1 FROM novel_object_references r WHERE r.object_id=o.id)
		RETURNING o.id)
		INSERT INTO novel_object_events (object_id,event_type,detail)
		SELECT id,'orphaned',jsonb_build_object('reason','activation_failed') FROM changed`, id)
	if err != nil {
		return fmt.Errorf("abandon verified object after activation failure: %w", err)
	}
	return nil
}

// DeactivateWithTx removes the current reference, marks the object orphaned,
// and applies the owning soft-delete change in one transaction.
func (service *Service) DeactivateWithTx(ctx context.Context, target Target, apply func(*sql.Tx, Target) error) error {
	clean, err := normalizeTarget(target)
	if err != nil {
		return err
	}
	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	lockKey := fmt.Sprintf("%s:%d:%d", clean.Kind, clean.BookID, clean.OwnerID)
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return fmt.Errorf("lock object target for deactivation: %w", err)
	}
	var objectID sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT object_id FROM novel_object_references
		WHERE object_kind=$1 AND book_id=$2 AND owner_id=$3 FOR UPDATE`, clean.Kind, clean.BookID, clean.OwnerID).Scan(&objectID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("lock object reference for deactivation: %w", err)
	}
	if objectID.Valid {
		if _, err := tx.ExecContext(ctx, `DELETE FROM novel_object_references
			WHERE object_kind=$1 AND book_id=$2 AND owner_id=$3 AND object_id=$4`, clean.Kind, clean.BookID, clean.OwnerID, objectID.Int64); err != nil {
			return fmt.Errorf("remove object reference: %w", err)
		}
		result, err := tx.ExecContext(ctx, `UPDATE novel_objects SET state='orphaned',orphaned_at=now()
			WHERE id=$1 AND state='active'`, objectID.Int64)
		if err != nil {
			return fmt.Errorf("orphan deactivated object: %w", err)
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			return errors.New("deactivated object is not active")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO novel_object_events (object_id,event_type,detail)
			VALUES ($1,'orphaned',jsonb_build_object('reason','business_deleted'))`, objectID.Int64); err != nil {
			return fmt.Errorf("record deactivated object orphan: %w", err)
		}
	}
	if apply != nil {
		if err := apply(tx, clean); err != nil {
			return fmt.Errorf("apply object deactivation business change: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit object deactivation: %w", err)
	}
	return nil
}

func (service *Service) ReadActive(ctx context.Context, target Target) ([]byte, Object, error) {
	clean, err := normalizeTarget(target)
	if err != nil {
		return nil, Object{}, err
	}
	var object Object
	err = service.db.QueryRowContext(ctx, `SELECT o.id,o.version,o.object_key,o.sha256,o.byte_size,o.content_type,o.state,o.created_at
		FROM novel_object_references r JOIN novel_objects o ON o.id=r.object_id
		WHERE r.object_kind=$1 AND r.book_id=$2 AND r.owner_id=$3 AND o.state='active'`, clean.Kind, clean.BookID, clean.OwnerID).
		Scan(&object.ID, &object.Version, &object.Key, &object.SHA256, &object.ByteSize, &object.ContentType, &object.State, &object.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, Object{}, fmt.Errorf("%w for %s %d", ErrActiveObjectNotFound, clean.Kind, clean.OwnerID)
	}
	if err != nil {
		return nil, Object{}, fmt.Errorf("load active object: %w", err)
	}
	object.Target = clean
	body, err := service.blobs.Get(ctx, object.Key)
	if err != nil {
		return nil, Object{}, err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, object.ByteSize+1))
	if err != nil {
		return nil, Object{}, fmt.Errorf("read active object: %w", err)
	}
	digest := sha256.Sum256(data)
	if int64(len(data)) != object.ByteSize || hex.EncodeToString(digest[:]) != object.SHA256 {
		return nil, Object{}, errors.New("active object integrity check failed")
	}
	return data, object, nil
}

func (service *Service) Collect(ctx context.Context, before time.Time, limit int) (CollectResult, error) {
	if limit < 1 || limit > 1000 {
		return CollectResult{}, errors.New("collect limit must be between 1 and 1000")
	}
	rows, err := service.db.QueryContext(ctx, `SELECT o.id,o.object_key FROM novel_objects o
		WHERE o.state IN ('uploading','orphaned','failed','deleting')
		AND COALESCE(o.orphaned_at,o.created_at) < $1
		AND NOT EXISTS (SELECT 1 FROM novel_object_references r WHERE r.object_id=o.id)
		ORDER BY COALESCE(o.orphaned_at,o.created_at),o.id LIMIT $2`, before, limit)
	if err != nil {
		return CollectResult{}, fmt.Errorf("list collectible objects: %w", err)
	}
	type candidate struct {
		id  int64
		key string
	}
	candidates := make([]candidate, 0, limit)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.key); err != nil {
			rows.Close()
			return CollectResult{}, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Close(); err != nil {
		return CollectResult{}, err
	}
	result := CollectResult{Examined: len(candidates)}
	for _, item := range candidates {
		deleted, err := service.collectOne(ctx, item.id, item.key)
		if err != nil {
			result.Failed++
			continue
		}
		if deleted {
			result.Deleted++
		}
	}
	return result, nil
}

func (service *Service) collectOne(ctx context.Context, id int64, key string) (bool, error) {
	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var state string
	var referenced bool
	err = tx.QueryRowContext(ctx, `SELECT o.state,EXISTS(SELECT 1 FROM novel_object_references r WHERE r.object_id=o.id)
		FROM novel_objects o WHERE o.id=$1 AND o.object_key=$2 FOR UPDATE`, id, key).Scan(&state, &referenced)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock collectible object: %w", err)
	}
	if referenced || (state != StateUploading && state != StateOrphaned && state != StateFailed && state != StateDeleting) {
		return false, nil
	}
	if state != StateDeleting {
		if _, err := tx.ExecContext(ctx, `UPDATE novel_objects SET state='deleting' WHERE id=$1`, id); err != nil {
			return false, fmt.Errorf("claim collectible object: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO novel_object_events (object_id,event_type,detail)
			VALUES ($1,'delete_started',jsonb_build_object('previousState',$2::text))`, id, state); err != nil {
			return false, fmt.Errorf("record object deletion start: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit object deletion claim: %w", err)
	}
	if err := service.blobs.Remove(ctx, key); err != nil {
		_, restoreErr := service.db.ExecContext(ctx, `WITH restored AS (
			UPDATE novel_objects SET state='failed',error_code='DELETE_FAILED' WHERE id=$1 AND state='deleting' RETURNING id)
			INSERT INTO novel_object_events (object_id,event_type) SELECT id,'delete_failed' FROM restored`, id)
		if restoreErr != nil {
			return false, errors.Join(err, fmt.Errorf("restore object after delete failure: %w", restoreErr))
		}
		return false, err
	}
	result, err := service.db.ExecContext(ctx, `WITH deleted AS (
		UPDATE novel_objects SET state='deleted',deleted_at=now(),error_code=NULL
		WHERE id=$1 AND state='deleting' RETURNING id)
		INSERT INTO novel_object_events (object_id,event_type) SELECT id,'deleted' FROM deleted`, id)
	if err != nil {
		return false, fmt.Errorf("mark collected object deleted: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return false, errors.New("object deletion claim was lost")
	}
	return true, nil
}

func SafeExtension(name string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
}
