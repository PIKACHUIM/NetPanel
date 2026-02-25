module github.com/netpanel/netpanel

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/gin-contrib/cors v1.5.0
	github.com/gin-contrib/gzip v0.0.6
	gorm.io/gorm v1.25.5
	gorm.io/driver/sqlite v1.5.4
	github.com/golang-jwt/jwt/v5 v5.2.0
	github.com/sirupsen/logrus v1.9.3
	github.com/google/uuid v1.4.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/shirou/gopsutil/v3 v3.23.10
	golang.org/x/net v0.19.0
	golang.org/x/crypto v0.17.0
	github.com/miekg/dns v1.1.57
	github.com/fatedier/golib v0.3.1
	github.com/go-acme/lego/v4 v4.14.2
	github.com/pkg/sftp v1.13.6
	github.com/hirochachacha/go-smb2 v1.1.0
)
