// Package wire contains the frozen Reader API serialization conventions.
package wire

import "time"

const DateTimeLayout = "2006-01-02 15:04:05"

func DateTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC().Format(DateTimeLayout)
}

func DateTimePointer(value *time.Time) any {
	if value == nil {
		return nil
	}
	return DateTime(*value)
}
