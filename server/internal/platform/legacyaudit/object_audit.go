package legacyaudit

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
)

const maxObjectIssueSamples = 100

type ObjectIntegrityReport struct {
	Checked    int64            `json:"checked"`
	Bytes      int64            `json:"bytes"`
	IssueCount int64            `json:"issueCount"`
	Issues     map[string]int64 `json:"issues,omitempty"`
	Samples    []ObjectIssue    `json:"samples,omitempty"`
}

type ObjectIssue struct {
	Code        string `json:"code"`
	Fingerprint string `json:"fingerprint"`
}

type objectCandidate struct {
	identity                        string
	kind, expectedKind              string
	bookID, ownerID                 int64
	expectedBookID, expectedOwnerID int64
	key, sha256                     string
	byteSize                        int64
}

type objectVerifier interface {
	Stat(context.Context, string) (objectstore.BlobStat, error)
	Get(context.Context, string) (io.ReadCloser, error)
}

func AuditObjects(ctx context.Context, db *sql.DB, blobs objectVerifier) (ObjectIntegrityReport, error) {
	if db == nil || blobs == nil {
		return ObjectIntegrityReport{}, fmt.Errorf("target database and object store are required")
	}
	report := ObjectIntegrityReport{Issues: map[string]int64{}}
	rows, err := db.QueryContext(ctx, referencedObjectQuery)
	if err != nil {
		return report, fmt.Errorf("query referenced objects: %w", err)
	}
	for rows.Next() {
		var candidate objectCandidate
		if err := rows.Scan(&candidate.identity, &candidate.kind, &candidate.bookID, &candidate.ownerID, &candidate.key, &candidate.sha256, &candidate.byteSize, &candidate.expectedKind, &candidate.expectedBookID, &candidate.expectedOwnerID); err != nil {
			rows.Close()
			return report, fmt.Errorf("scan referenced object: %w", err)
		}
		auditObject(ctx, blobs, candidate, &report)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return report, fmt.Errorf("iterate referenced objects: %w", err)
	}
	rows.Close()

	txtRows, err := db.QueryContext(ctx, `SELECT id::text,target_book_id,COALESCE(object_key,''),COALESCE(object_sha256,''),COALESCE(object_byte_size,0) FROM novel_txt_import_task ORDER BY id`)
	if err != nil {
		return report, fmt.Errorf("query TXT objects: %w", err)
	}
	for txtRows.Next() {
		var candidate objectCandidate
		if err := txtRows.Scan(&candidate.identity, &candidate.bookID, &candidate.key, &candidate.sha256, &candidate.byteSize); err != nil {
			txtRows.Close()
			return report, fmt.Errorf("scan TXT object: %w", err)
		}
		if candidate.key == "" || candidate.sha256 == "" || candidate.byteSize <= 0 {
			continue
		}
		candidate.kind, candidate.expectedKind = "txt_import", "txt_import"
		candidate.ownerID, candidate.expectedBookID, candidate.expectedOwnerID = parseIdentity(candidate.identity), candidate.bookID, parseIdentity(candidate.identity)
		auditObject(ctx, blobs, candidate, &report)
	}
	if err := txtRows.Err(); err != nil {
		txtRows.Close()
		return report, fmt.Errorf("iterate TXT objects: %w", err)
	}
	txtRows.Close()
	sort.Slice(report.Samples, func(i, j int) bool {
		if report.Samples[i].Code == report.Samples[j].Code {
			return report.Samples[i].Fingerprint < report.Samples[j].Fingerprint
		}
		return report.Samples[i].Code < report.Samples[j].Code
	})
	if report.IssueCount == 0 {
		report.Issues = nil
	}
	return report, nil
}

func auditObject(ctx context.Context, blobs objectVerifier, candidate objectCandidate, report *ObjectIntegrityReport) {
	report.Checked++
	report.Bytes += candidate.byteSize
	if candidate.kind != candidate.expectedKind || candidate.bookID != candidate.expectedBookID || candidate.ownerID != candidate.expectedOwnerID {
		recordObjectIssue(report, "REFERENCE_METADATA_MISMATCH", candidate)
	}
	stat, err := blobs.Stat(ctx, candidate.key)
	if err != nil {
		recordObjectIssue(report, "OBJECT_MISSING", candidate)
		return
	}
	if stat.ByteSize != candidate.byteSize {
		recordObjectIssue(report, "STAT_SIZE_MISMATCH", candidate)
	}
	if !strings.EqualFold(stat.SHA256, candidate.sha256) {
		recordObjectIssue(report, "STAT_HASH_MISMATCH", candidate)
	}
	body, err := blobs.Get(ctx, candidate.key)
	if err != nil {
		recordObjectIssue(report, "OBJECT_READ_FAILED", candidate)
		return
	}
	hash := sha256.New()
	readBytes, readErr := io.Copy(hash, io.LimitReader(body, candidate.byteSize+1))
	closeErr := body.Close()
	if readErr != nil || closeErr != nil {
		recordObjectIssue(report, "OBJECT_READ_FAILED", candidate)
		return
	}
	if readBytes != candidate.byteSize {
		recordObjectIssue(report, "CONTENT_SIZE_MISMATCH", candidate)
		return
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), candidate.sha256) {
		recordObjectIssue(report, "CONTENT_HASH_MISMATCH", candidate)
	}
}

func recordObjectIssue(report *ObjectIntegrityReport, code string, candidate objectCandidate) {
	report.IssueCount++
	report.Issues[code]++
	if len(report.Samples) >= maxObjectIssueSamples {
		return
	}
	digest := sha256.Sum256([]byte(candidate.kind + "\x00" + candidate.identity))
	report.Samples = append(report.Samples, ObjectIssue{Code: code, Fingerprint: hex.EncodeToString(digest[:])})
}

func parseIdentity(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

const referencedObjectQuery = `
WITH business_references(object_id,expected_kind,expected_book_id,expected_owner_id) AS (
    SELECT object_id,object_kind,book_id,owner_id FROM novel_object_references
    UNION ALL
    SELECT cleaned_object_id,'chapter_clean',book_id,chapter_id FROM novel_chapter_clean_result WHERE cleaned_object_id IS NOT NULL
    UNION ALL
    SELECT original_object_id,'chapter_content',book_id,chapter_id FROM novel_chapter_clean_result WHERE original_object_id IS NOT NULL
    UNION ALL
    SELECT source_object_id,'chapter_content',source_book_id,source_chapter_id FROM novel_book_merge_chapter WHERE source_object_id IS NOT NULL
    UNION ALL
    SELECT target_object_id,'chapter_content',target_book_id,target_chapter_id FROM novel_book_merge_chapter WHERE target_object_id IS NOT NULL
)
SELECT o.id::text,o.object_kind,o.book_id,o.owner_id,o.object_key,o.sha256,o.byte_size,
       r.expected_kind,r.expected_book_id,r.expected_owner_id
FROM business_references r
JOIN novel_objects o ON o.id=r.object_id
WHERE o.state <> 'deleted'
ORDER BY o.id,r.expected_kind,r.expected_book_id,r.expected_owner_id`
