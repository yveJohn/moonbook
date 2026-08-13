package apperror

import (
	"errors"
	"net/http"
	"testing"
)

func TestExposeReturnsStablePublicErrorAndPreservesCause(t *testing.T) {
	cause := errors.New("postgres://user:secret@internal/database")
	err := Wrap(cause, CodeUnavailable, http.StatusServiceUnavailable, "service temporarily unavailable")
	if !errors.Is(err, cause) {
		t.Fatal("wrapped cause is not preserved")
	}
	public := Expose(err)
	if public.Code != CodeUnavailable || public.HTTPStatus != http.StatusServiceUnavailable || public.Message != "service temporarily unavailable" {
		t.Fatalf("public error = %+v", public)
	}
}

func TestExposeRedactsUnknownError(t *testing.T) {
	public := Expose(errors.New("secret database detail"))
	if public.Code != CodeInternal || public.HTTPStatus != http.StatusInternalServerError || public.Message != "internal service error" {
		t.Fatalf("public error = %+v", public)
	}
}
