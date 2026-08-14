package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/adminpayment"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/adminrecharge"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/catalog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/checkin"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/payment"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/purchase"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/recharge"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/wallet"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/books"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/chapters"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/metadata"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/readerseo"
	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	readerinvite "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/invite"
	readerme "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/me"
	readerpublic "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/public"
	"github.com/flipped-aurora/gin-vue-admin/server/router"
	"github.com/gin-gonic/gin"
	"os"
)

// 占位方法，保证文件可以正确加载，避免go空变量检测报错，请勿删除。
func holder(routers ...*gin.RouterGroup) {
	_ = routers
	_ = router.RouterGroupApp
}

func initBizRouter(routers ...*gin.RouterGroup) {
	privateGroup := routers[0]
	publicGroup := routers[1]

	holder(publicGroup, privateGroup)
	db, err := global.GVA_DB.DB()
	if err != nil {
		panic("open novel metadata database: " + err.Error())
	}
	metadata.RegisterRoutes(privateGroup, db)
	readerService := readerauth.NewService(readerauth.SQLRepository{DB: db}, readerauth.RedisRateLimiter{Client: global.GVA_REDIS, Prefix: "moonbook:reader:rate:"}, readerauth.TokenConfig{Secret: []byte(global.GVA_CONFIG.JWT.SigningKey)})
	registration := readerinvite.NewService(readerinvite.SQLRepository{DB: db}, readerService)
	readerauth.RegisterRoutes(publicGroup, readerauth.NewHandler(readerService, registration))
	readerme.RegisterRoutes(publicGroup, readerme.NewService(readerme.SQLRepository{DB: db}), readerService)
	wallet.RegisterRoutes(publicGroup, wallet.NewService(wallet.SQLRepository{DB: db}), readerService)
	recharge.RegisterRoutes(publicGroup, recharge.NewService(recharge.SQLRepository{DB: db}), readerService)
	checkin.RegisterRoutes(publicGroup, checkin.NewService(checkin.SQLRepository{DB: db}), readerService)
	purchase.RegisterRoutes(publicGroup, purchase.NewService(purchase.SQLRepository{DB: db}), readerService)
	payment.RegisterRoutes(publicGroup, payment.NewService(payment.SQLRepository{DB: db}, os.Getenv("MOONBOOK_EPUSDT_PID"), os.Getenv("MOONBOOK_EPUSDT_SECRET")))
	adminrecharge.RegisterRoutes(privateGroup, adminrecharge.NewService(adminrecharge.SQLRepository{DB: db}))
	adminpayment.RegisterRoutes(privateGroup, adminpayment.NewService(adminpayment.SQLRepository{DB: db}))
	commerce := catalog.NewService(catalog.SQLRepository{DB: db})
	minio := global.GVA_CONFIG.Minio
	blobs, err := objectstore.NewMinIOStore(objectstore.MinIOConfig{Endpoint: minio.Endpoint, AccessKey: minio.AccessKeyId, SecretKey: minio.AccessKeySecret, Bucket: minio.BucketName, UseSSL: minio.UseSSL})
	if err != nil {
		panic("initialize novel object store: " + err.Error())
	}
	objects := objectstore.NewService(db, blobs)
	books.RegisterRoutes(privateGroup, db, objects)
	chapters.RegisterRoutes(privateGroup, db, objects)
	readerseo.RegisterRoutes(privateGroup, db)
	readerpublic.RegisterRoutes(publicGroup, db, objects, readerService, commerce)

}
