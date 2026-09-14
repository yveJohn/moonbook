package serverchan

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type configLoaderStub struct {
	config Config
	err    error
}

func (stub configLoaderStub) Load(context.Context) (Config, error) { return stub.config, stub.err }

type senderStub struct {
	calls       int
	sendKey     string
	title       string
	description string
	err         error
	ctxErr      error
}

func (stub *senderStub) Send(ctx context.Context, sendKey, title, description string) error {
	stub.calls++
	stub.sendKey, stub.title, stub.description, stub.ctxErr = sendKey, title, description, ctx.Err()
	return stub.err
}

func TestNotifierSkipsDisabledAndFormatsFeedback(t *testing.T) {
	sender := &senderStub{}
	notifier := NewNotifier(configLoaderStub{}, sender)
	if err := notifier.NotifyFeedback(context.Background(), 1, 2, time.Now(), "内容"); err != nil || sender.calls != 0 {
		t.Fatalf("disabled calls=%d err=%v", sender.calls, err)
	}

	notifier = NewNotifier(configLoaderStub{config: Config{Enabled: true, SendKey: "SCT_test_secret_123456"}}, sender)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	content := strings.Repeat("月", 1001) + "\n末尾"
	createdAt := time.Date(2026, 9, 14, 11, 30, 0, 0, time.FixedZone("MYT", 8*60*60))
	if err := notifier.NotifyFeedback(canceled, 9223372036854775807, 9007199254740993, createdAt, content); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Moonbook 新用户反馈", "9223372036854775807", "9007199254740993", "2026-09-14 11:30:00 MYT", strings.Repeat("月", 1000), "已截断"} {
		if !strings.Contains(sender.title+sender.description, expected) {
			t.Fatalf("notification missing %q", expected)
		}
	}
	if strings.Contains(sender.description, "末尾") || sender.ctxErr != nil {
		t.Fatalf("description was not truncated or context stayed canceled: ctx=%v", sender.ctxErr)
	}
}

func TestNotifierReturnsConfigurationAndSenderErrors(t *testing.T) {
	want := errors.New("send failed")
	sender := &senderStub{err: want}
	notifier := NewNotifier(configLoaderStub{err: ErrConfiguration}, sender)
	if err := notifier.NotifyFeedback(context.Background(), 1, 2, time.Now(), "内容"); !errors.Is(err, ErrConfiguration) || sender.calls != 0 {
		t.Fatalf("config err=%v calls=%d", err, sender.calls)
	}
	notifier = NewNotifier(configLoaderStub{config: Config{Enabled: true, SendKey: "SCT_test_secret_123456"}}, sender)
	if err := notifier.NotifyFeedback(context.Background(), 1, 2, time.Now(), "内容"); !errors.Is(err, want) || sender.calls != 1 {
		t.Fatalf("sender err=%v calls=%d", err, sender.calls)
	}
}
