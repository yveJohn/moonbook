package system

import (
	"context"
	"errors"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/logger"
)

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetSystemConfig
//@description: 读取配置文件
//@return: conf config.Server, err error

type SystemConfigService struct{}

var SystemConfigServiceApp = new(SystemConfigService)

func (systemConfigService *SystemConfigService) GetSystemConfig(ctx context.Context) (map[string]any, error) {
	return publicSystemConfig(global.GVA_CONFIG), nil
}

// @description   set system config,
//@author: [piexlmax](https://github.com/piexlmax)
//@function: SetSystemConfig
//@description: 设置配置文件
//@param: system model.System
//@return: err error

func (systemConfigService *SystemConfigService) SetSystemConfig(ctx context.Context, system system.System) (err error) {
	return errors.New("系统配置由环境变量管理，禁止通过管理 API 修改")
}

// publicSystemConfig is deliberately assembled field by field. Reflecting the
// full Server config would make a future secret field an accidental API leak.
func publicSystemConfig(c config.Server) map[string]any {
	return map[string]any{
		"system": map[string]any{
			"db-type": c.System.DbType, "oss-type": c.System.OssType, "router-prefix": c.System.RouterPrefix,
			"addr": c.System.Addr, "iplimit-count": c.System.LimitCountIP, "iplimit-time": c.System.LimitTimeIP,
			"use-multipoint": c.System.UseMultipoint, "use-redis": c.System.UseRedis, "use-mongo": c.System.UseMongo,
			"use-strict-auth": c.System.UseStrictAuth, "disable-auto-migrate": c.System.DisableAutoMigrate,
		},
		"app": map[string]any{"node": c.App.Node, "app-id": c.App.AppID, "env": c.App.Env},
		"jwt": map[string]any{
			"expires-time": c.JWT.ExpiresTime, "buffer-time": c.JWT.BufferTime, "issuer": c.JWT.Issuer,
			"signing-key-configured": c.JWT.SigningKey != "",
		},
		"redis": map[string]any{
			"name": c.Redis.Name, "addr": c.Redis.Addr, "db": c.Redis.DB, "useCluster": c.Redis.UseCluster,
			"clusterAddrs": c.Redis.ClusterAddrs, "password-configured": c.Redis.Password != "",
		},
		"metrics": map[string]any{"enabled": c.Metrics.Enabled, "token-configured": c.Metrics.Token != ""},
		"minio": map[string]any{
			"endpoint": c.Minio.Endpoint, "bucket-name": c.Minio.BucketName, "use-ssl": c.Minio.UseSSL,
			"base-path": c.Minio.BasePath, "bucket-url": c.Minio.BucketUrl,
			"access-key-configured": c.Minio.AccessKeyId != "", "secret-configured": c.Minio.AccessKeySecret != "",
		},
		"pgsql": publicDBConfig(c.Pgsql.GeneralDB),
		"zap":   map[string]any{"level": c.Zap.Level, "format": c.Zap.Format, "encode-level": c.Zap.EncodeLevel, "stacktrace-key": c.Zap.StacktraceKey, "prefix": c.Zap.Prefix, "director": c.Zap.Director, "retention-day": c.Zap.RetentionDay, "show-line": c.Zap.ShowLine, "log-in-console": c.Zap.LogInConsole},
		"cors":  map[string]any{"mode": c.Cors.Mode, "whitelist": c.Cors.Whitelist},
	}
}

func publicDBConfig(db config.GeneralDB) map[string]any {
	return map[string]any{
		"username": db.Username, "path": db.Path, "port": db.Port, "db-name": db.Dbname, "prefix": db.Prefix,
		"engine": db.Engine, "max-idle-conns": db.MaxIdleConns, "max-open-conns": db.MaxOpenConns,
		"conn-max-lifetime": db.ConnMaxLifetime, "singular": db.Singular, "log-mode": db.LogMode,
		"password-configured": db.Password != "",
	}
}

//@author: [SliverHorn](https://github.com/SliverHorn)
//@function: GetServerInfo
//@description: 获取服务器信息
//@return: server *utils.Server, err error

func (systemConfigService *SystemConfigService) GetServerInfo(ctx context.Context) (server *utils.Server, err error) {
	var s utils.Server
	s.Os = utils.InitOS()
	if s.Cpu, err = utils.InitCPU(); err != nil {
		logger.WithCtx(ctx).Mod("biz").Field("err", err.Error()).Error("func utils.InitCPU() Failed")
		return &s, err
	}
	if s.Ram, err = utils.InitRAM(); err != nil {
		logger.WithCtx(ctx).Mod("biz").Field("err", err.Error()).Error("func utils.InitRAM() Failed")
		return &s, err
	}
	if s.Disk, err = utils.InitDisk(); err != nil {
		logger.WithCtx(ctx).Mod("biz").Field("err", err.Error()).Error("func utils.InitDisk() Failed")
		return &s, err
	}

	return &s, nil
}
