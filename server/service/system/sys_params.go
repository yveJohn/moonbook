package system

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/secretcrypto"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/serverchan"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SysParamsService struct{}

var (
	ErrReservedParamDuplicate = errors.New("Server酱保留参数已存在")
	ErrReservedParamRename    = errors.New("Server酱保留参数键不允许改名")
	ErrInvalidServerChanValue = errors.New("Server酱参数值不合法")
)

// CreateSysParams 创建参数记录
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) CreateSysParams(ctx context.Context, sysParams *system.SysParams) (err error) {
	if sysParams == nil {
		return ErrInvalidServerChanValue
	}
	if !isServerChanParam(sysParams.Key) {
		return global.GVA_DB.WithContext(ctx).Create(sysParams).Error
	}
	return global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "moonbook:sys_params:"+sysParams.Key).Error; err != nil {
				return err
			}
		}
		var count int64
		if err := tx.Model(&system.SysParams{}).Where("key = ?", sysParams.Key).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrReservedParamDuplicate
		}
		if err := protectServerChanValue(sysParams); err != nil {
			return err
		}
		return tx.Create(sysParams).Error
	})
}

// DeleteSysParams 删除参数记录
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) DeleteSysParams(ctx context.Context, ID string) (err error) {
	err = global.GVA_DB.WithContext(ctx).Delete(&system.SysParams{}, "id = ?", ID).Error
	return err
}

// DeleteSysParamsByIds 批量删除参数记录
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) DeleteSysParamsByIds(ctx context.Context, IDs []string) (err error) {
	err = global.GVA_DB.WithContext(ctx).Delete(&[]system.SysParams{}, "id in ?", IDs).Error
	return err
}

// UpdateSysParams 更新参数记录
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) UpdateSysParams(ctx context.Context, sysParams system.SysParams) (err error) {
	return global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current system.SysParams
		if err := tx.Where("id = ?", sysParams.ID).First(&current).Error; err != nil {
			return err
		}
		if current.Key != sysParams.Key && (isServerChanParam(current.Key) || isServerChanParam(sysParams.Key)) {
			return ErrReservedParamRename
		}
		if isServerChanParam(current.Key) {
			if current.Key == serverchan.SendKeyParamKey && sysParams.Value == serverchan.SecretMask {
				sysParams.Value = current.Value
			} else if err := protectServerChanValue(&sysParams); err != nil {
				return err
			}
		}
		return tx.Model(&system.SysParams{}).Where("id = ?", sysParams.ID).Updates(&sysParams).Error
	})
}

// GetSysParams 根据ID获取参数记录
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) GetSysParams(ctx context.Context, ID string) (sysParams system.SysParams, err error) {
	err = global.GVA_DB.WithContext(ctx).Where("id = ?", ID).First(&sysParams).Error
	maskServerChanSecret(&sysParams)
	return
}

// GetSysParamsInfoList 分页获取参数记录
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) GetSysParamsInfoList(ctx context.Context, info systemReq.SysParamsSearch) (list []system.SysParams, total int64, err error) {
	limit, offset := info.LimitOffset()
	// 创建db
	db := global.GVA_DB.WithContext(ctx).Model(&system.SysParams{})
	var sysParamss []system.SysParams
	// 如果有条件搜索 下方会自动创建搜索语句
	if info.StartCreatedAt != nil && info.EndCreatedAt != nil {
		db = db.Where("created_at BETWEEN ? AND ?", info.StartCreatedAt, info.EndCreatedAt)
	}
	if info.Name != "" {
		db = db.Where("name LIKE ?", "%"+info.Name+"%")
	}
	if info.Key != "" {
		db = db.Where(clause.Like{
			Column: clause.Column{Name: "key"},
			Value:  "%" + info.Key + "%",
		})
	}
	err = db.Count(&total).Error
	if err != nil {
		return
	}

	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	err = db.Find(&sysParamss).Error
	for i := range sysParamss {
		maskServerChanSecret(&sysParamss[i])
	}
	return sysParamss, total, err
}

// GetSysParam 根据key获取参数value
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) GetSysParam(ctx context.Context, key string) (param system.SysParams, err error) {
	err = global.GVA_DB.WithContext(ctx).Where(system.SysParams{Key: key}).First(&param).Error
	maskServerChanSecret(&param)
	return
}

func isServerChanParam(key string) bool {
	return key == serverchan.EnabledParamKey || key == serverchan.SendKeyParamKey
}

func protectServerChanValue(param *system.SysParams) error {
	value := strings.TrimSpace(param.Value)
	switch param.Key {
	case serverchan.EnabledParamKey:
		if value != "true" && value != "false" {
			return ErrInvalidServerChanValue
		}
		param.Value = value
		return nil
	case serverchan.SendKeyParamKey:
		if value == "" || value == serverchan.SecretMask {
			return ErrInvalidServerChanValue
		}
		cipher, err := secretcrypto.NewFromEnv(os.LookupEnv)
		if err != nil {
			return err
		}
		param.Value, err = cipher.Encrypt(value, serverchan.SendKeyScope())
		return err
	default:
		return nil
	}
}

func maskServerChanSecret(param *system.SysParams) {
	if param != nil && param.Key == serverchan.SendKeyParamKey {
		param.Value = serverchan.SecretMask
	}
}
