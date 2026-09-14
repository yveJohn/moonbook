package serverchan

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

const (
	feedbackTitle      = "Moonbook 新用户反馈"
	feedbackMaxRunes   = 1000
	notificationTimout = 3 * time.Second
)

type Notifier struct {
	configs ConfigLoader
	sender  Sender
}

func NewNotifier(configs ConfigLoader, sender Sender) *Notifier {
	return &Notifier{configs: configs, sender: sender}
}

func (notifier *Notifier) NotifyFeedback(ctx context.Context, feedbackID, readerID int64, createdAt time.Time, content string) error {
	if notifier == nil || notifier.configs == nil || notifier.sender == nil {
		return ErrConfiguration
	}
	if ctx == nil {
		ctx = context.Background()
	}
	notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), notificationTimout)
	defer cancel()
	config, err := notifier.configs.Load(notifyCtx)
	if err != nil || !config.Enabled {
		return err
	}
	description := feedbackDescription(feedbackID, readerID, createdAt, content)
	return notifier.sender.Send(notifyCtx, config.SendKey, feedbackTitle, description)
}

func feedbackDescription(feedbackID, readerID int64, createdAt time.Time, content string) string {
	runes := []rune(content)
	truncated := len(runes) > feedbackMaxRunes
	if truncated {
		runes = runes[:feedbackMaxRunes]
	}
	description := fmt.Sprintf(
		"- 反馈 ID：`%s`\n- 用户 ID：`%s`\n- 提交时间：%s\n\n## 反馈内容\n\n%s",
		strconv.FormatInt(feedbackID, 10),
		strconv.FormatInt(readerID, 10),
		createdAt.Format("2006-01-02 15:04:05 MST"),
		string(runes),
	)
	if truncated {
		description += "\n\n> 内容超过 1000 个字符，已截断。"
	}
	return description
}
