package main

import (
	"embed"
	"log/slog"
	"net/http"
	"sharedmodule"
	"text/template"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	smn "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/smn/v2"
	smnRegion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/smn/v2/region"
	"github.com/joho/godotenv"
)

//go:embed templates/*
var templateFiles embed.FS

type Config struct {
	AccessKey               string
	SecretKey               string
	SmnTopicNotificationUrn string
	SmnTopicHostingUrn      string
	RootDomainName          string
}

var appConfig Config

func loadConfig() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found")
	}

	appConfig.AccessKey = sharedmodule.GetEnv("ACCESS_KEY", "")
	appConfig.SecretKey = sharedmodule.GetEnv("SECRET_KEY", "")
	appConfig.SmnTopicNotificationUrn = sharedmodule.GetEnv("SMN_TOPIC_NOTIFICATION_URN", "notification")
	appConfig.SmnTopicHostingUrn = sharedmodule.GetEnv("SMN_TOPIC_HOSTING_URN", "newhosting")
	appConfig.RootDomainName = sharedmodule.GetEnv("ROOT_DOMAIN_NAME", "onhuawei.cloud")
}

func main() {

	loadConfig()

	cred, err := basic.NewCredentialsBuilder().WithAk(appConfig.AccessKey).WithSk(appConfig.SecretKey).SafeBuild()
	if err != nil {
		panic(err)
	}

	smnHcClient, err := smn.SmnClientBuilder().WithCredential(cred).WithRegion(smnRegion.AP_SOUTHEAST_4).SafeBuild()
	if err != nil {
		panic(err)
	}

	smnClient := smn.NewSmnClient(smnHcClient)

	tmpl := template.Must(template.ParseFS(templateFiles, "templates/*"))
	h := &handler{
		tmpl:      tmpl,
		smnClient: smnClient,
	}

	http.HandleFunc("/", h.indexHandler)
	http.HandleFunc("/submit", h.submitHandler)

	slog.Info("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
