package serverchan

import "github.com/flipped-aurora/gin-vue-admin/server/internal/platform/secretcrypto"

const (
	EnabledParamKey = "serverchan.enabled"
	SendKeyParamKey = "serverchan.send_key"
	SecretMask      = "********"
)

func SendKeyScope() secretcrypto.Scope {
	return secretcrypto.Scope{Table: "sys_params", RecordID: SendKeyParamKey, Field: "value"}
}
