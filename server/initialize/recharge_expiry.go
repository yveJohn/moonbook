package initialize

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/adminpayment"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/recharge"
	"go.uber.org/zap"
)

const (
	rechargeExpiryBatchSize = 500
	rechargeExpiryInterval  = time.Minute
)

type rechargeExpiryRunner interface {
	Run(context.Context) error
}

var rechargeExpiry struct {
	sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

var buildRechargeExpiryRunner = func() (rechargeExpiryRunner, error) {
	if global.GVA_DB == nil {
		return nil, fmt.Errorf("recharge expiry database is not initialized")
	}
	db, err := global.GVA_DB.DB()
	if err != nil {
		return nil, fmt.Errorf("open recharge expiry database: %w", err)
	}
	store := adminpayment.SQLRepository{DB: db}
	return &recharge.ExpiryWorker{
		Repo:       recharge.SQLRepository{DB: db},
		BatchSize:  rechargeExpiryBatchSize,
		LoadWindow: store.UnknownReleaseWindow,
		Interval:   rechargeExpiryInterval,
		OnError: func(err error) {
			zap.L().Error("充值订单过期扫描失败", zap.Error(err))
		},
	}, nil
}

func StartRechargeExpiry() error {
	StopRechargeExpiry()
	runner, err := buildRechargeExpiryRunner()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	rechargeExpiry.Lock()
	rechargeExpiry.cancel, rechargeExpiry.done = cancel, done
	rechargeExpiry.Unlock()
	go func() {
		defer close(done)
		if err := runner.Run(ctx); err != nil {
			zap.L().Error("充值订单过期任务停止", zap.Error(err))
		}
	}()
	return nil
}

func StopRechargeExpiry() {
	rechargeExpiry.Lock()
	cancel, done := rechargeExpiry.cancel, rechargeExpiry.done
	rechargeExpiry.cancel, rechargeExpiry.done = nil, nil
	rechargeExpiry.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			zap.L().Warn("等待充值订单过期任务停止超时")
		}
	}
}
