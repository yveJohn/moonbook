package payment

import (
	"testing"
	"time"
)

func TestLoadAuditConfigValidatesCallbackStaleMinutes(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		set     bool
		want    time.Duration
		wantErr bool
	}{
		{name: "default", want: 5 * time.Minute},
		{name: "minimum", value: "1", set: true, want: time.Minute},
		{name: "maximum", value: "1440", set: true, want: 24 * time.Hour},
		{name: "empty configured value", value: "", set: true, wantErr: true},
		{name: "zero", value: "0", set: true, wantErr: true},
		{name: "above maximum", value: "1441", set: true, wantErr: true},
		{name: "not an integer", value: "five", set: true, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config, err := LoadAuditConfig(func(key string) (string, bool) {
				if key != EnvCallbackStaleMinutes || !test.set {
					return "", false
				}
				return test.value, true
			})
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, config=%+v", config)
				}
				return
			}
			if err != nil || config.StaleAfter != test.want {
				t.Fatalf("config=%+v err=%v want=%s", config, err, test.want)
			}
		})
	}
}
