package legacyaudit

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
)

type fakeObjectVerifier struct {
	data    map[string][]byte
	stats   map[string]objectstore.BlobStat
	getErr  map[string]error
	statErr map[string]error
}

func (v fakeObjectVerifier) Stat(_ context.Context, key string) (objectstore.BlobStat, error) {
	if err := v.statErr[key]; err != nil {
		return objectstore.BlobStat{}, err
	}
	return v.stats[key], nil
}

func (v fakeObjectVerifier) Get(_ context.Context, key string) (io.ReadCloser, error) {
	if err := v.getErr[key]; err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(v.data[key])), nil
}

func TestAuditObjectClassifiesIntegrityFailuresWithoutExposingKeys(t *testing.T) {
	verifier := fakeObjectVerifier{
		data:    map[string][]byte{"private/key": []byte("wrong")},
		stats:   map[string]objectstore.BlobStat{"private/key": {ByteSize: 5, SHA256: "different"}},
		statErr: map[string]error{}, getErr: map[string]error{},
	}
	report := ObjectIntegrityReport{Issues: map[string]int64{}}
	auditObject(context.Background(), verifier, objectCandidate{
		identity: "9007199254740993", kind: "chapter_content", expectedKind: "chapter_clean",
		bookID: 1, ownerID: 2, expectedBookID: 1, expectedOwnerID: 3,
		key: "private/key", sha256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", byteSize: 4,
	}, &report)
	for _, code := range []string{"REFERENCE_METADATA_MISMATCH", "STAT_SIZE_MISMATCH", "STAT_HASH_MISMATCH", "CONTENT_SIZE_MISMATCH"} {
		if report.Issues[code] != 1 {
			t.Fatalf("issues=%v missing=%s", report.Issues, code)
		}
	}
	if len(report.Samples) != 4 || report.Samples[0].Fingerprint == "" {
		t.Fatalf("samples=%+v", report.Samples)
	}
	for _, sample := range report.Samples {
		if sample.Fingerprint == "private/key" || sample.Fingerprint == "9007199254740993" {
			t.Fatal("sample exposed object identity")
		}
	}
}

func TestAuditObjectDetectsMissingAndReadFailures(t *testing.T) {
	for _, test := range []struct {
		name, code string
		verifier   fakeObjectVerifier
	}{
		{"missing", "OBJECT_MISSING", fakeObjectVerifier{statErr: map[string]error{"key": errors.New("missing")}}},
		{"read", "OBJECT_READ_FAILED", fakeObjectVerifier{stats: map[string]objectstore.BlobStat{"key": {ByteSize: 1}}, getErr: map[string]error{"key": errors.New("read")}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := ObjectIntegrityReport{Issues: map[string]int64{}}
			auditObject(context.Background(), test.verifier, objectCandidate{identity: "1", kind: "txt_import", expectedKind: "txt_import", bookID: 1, expectedBookID: 1, ownerID: 1, expectedOwnerID: 1, key: "key", byteSize: 1}, &report)
			if report.Issues[test.code] != 1 {
				t.Fatalf("issues=%v", report.Issues)
			}
		})
	}
}
