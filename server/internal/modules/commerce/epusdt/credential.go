package epusdt

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const (
	maxCredentialHistoryBytes = 64 << 10
	maxHistoricalCredentials  = 32
	maxMerchantPIDLength      = 255
	maxSecretLength           = 4096
)

var credentialRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

type Credential struct {
	ref    string
	pid    string
	secret string
}

func (credential Credential) Ref() string    { return credential.ref }
func (credential Credential) PID() string    { return credential.pid }
func (credential Credential) Secret() string { return credential.secret }
func (credential Credential) String() string { return "Credential{redacted}" }
func (credential Credential) GoString() string {
	return credential.String()
}

type CredentialProvider struct {
	current      Credential
	verification map[string]Credential
}

func NewCredentialProvider(currentRef, currentPID, currentSecret, historyJSON string) (*CredentialProvider, error) {
	current, err := validateCredential(currentRef, currentPID, currentSecret)
	if err != nil {
		return nil, fmt.Errorf("invalid EPUSDT current credential: %w", err)
	}
	history, err := decodeHistoricalCredentials(historyJSON)
	if err != nil {
		return nil, err
	}
	verification := make(map[string]Credential, len(history)+1)
	verification[current.ref] = current
	for _, credential := range history {
		if _, exists := verification[credential.ref]; exists {
			return nil, errors.New("EPUSDT credential references must be unique")
		}
		verification[credential.ref] = credential
	}
	return &CredentialProvider{current: current, verification: verification}, nil
}

func (provider *CredentialProvider) Current() Credential {
	if provider == nil {
		return Credential{}
	}
	return provider.current
}

func (provider *CredentialProvider) Verification(ref string) (Credential, bool) {
	if provider == nil {
		return Credential{}, false
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return provider.current, true
	}
	credential, ok := provider.verification[ref]
	return credential, ok
}

func (provider *CredentialProvider) String() string {
	if provider == nil {
		return "CredentialProvider{disabled}"
	}
	return fmt.Sprintf("CredentialProvider{redacted,count=%d}", len(provider.verification))
}

func (provider *CredentialProvider) GoString() string {
	return provider.String()
}

type credentialJSON struct {
	Ref    string `json:"ref"`
	PID    string `json:"pid"`
	Secret string `json:"secret"`
}

func decodeHistoricalCredentials(raw string) ([]Credential, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "[]"
	}
	if len(raw) > maxCredentialHistoryBytes {
		return nil, errors.New("EPUSDT verification credential JSON is too large")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var encoded []credentialJSON
	if err := decoder.Decode(&encoded); err != nil || encoded == nil {
		return nil, errors.New("EPUSDT verification credential JSON must be an object array")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("EPUSDT verification credential JSON must contain one array")
	}
	if len(encoded) > maxHistoricalCredentials {
		return nil, errors.New("EPUSDT verification credential history exceeds the limit")
	}
	credentials := make([]Credential, 0, len(encoded))
	for _, value := range encoded {
		credential, err := validateCredential(value.Ref, value.PID, value.Secret)
		if err != nil {
			return nil, fmt.Errorf("invalid EPUSDT historical credential: %w", err)
		}
		credentials = append(credentials, credential)
	}
	return credentials, nil
}

func validateCredential(ref, pid, secret string) (Credential, error) {
	ref = strings.TrimSpace(ref)
	pid = strings.TrimSpace(pid)
	if !credentialRefPattern.MatchString(ref) {
		return Credential{}, errors.New("credential reference is invalid")
	}
	if pid == "" || len(pid) > maxMerchantPIDLength {
		return Credential{}, errors.New("merchant PID is invalid")
	}
	if strings.TrimSpace(secret) == "" || len(secret) > maxSecretLength {
		return Credential{}, errors.New("credential secret is invalid")
	}
	return Credential{ref: ref, pid: pid, secret: secret}, nil
}
