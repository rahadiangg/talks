package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	"github.com/joho/godotenv"
	"huaweicloud.com/go-runtime/events/smn"
	fgcontext "huaweicloud.com/go-runtime/go-api/context"

	"sharedmodule"

	dns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	dnsModel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	dnsRegion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/region"
	"huaweicloud.com/go-runtime/pkg/runtime"
)

type Config struct {
	AccessKey   string
	SecretKey   string
	DnsZoneId   string
	DnsRecordIp string
	DnsZoneName string
}

var AppConfig Config

func loadConfig() {
	if err := godotenv.Load(); err != nil {
		fmt.Printf("failed to load .env file: %s\n", err.Error())
	}

	AppConfig.AccessKey = sharedmodule.GetEnv("ACCESS_KEY", "")
	AppConfig.SecretKey = sharedmodule.GetEnv("SECRET_KEY", "")
	AppConfig.DnsZoneId = sharedmodule.GetEnv("DNS_ZONE_ID", "")
	AppConfig.DnsRecordIp = sharedmodule.GetEnv("DNS_RECORD_IP", "")
	AppConfig.DnsZoneName = sharedmodule.GetEnv("DNS_ZONE_NAME", "onhuawei.cloud")
}

type handler struct {
	Client *dns.DnsClient
}

func (h *handler) SmnTrigger(payload []byte, ctx fgcontext.RuntimeContext) (interface{}, error) {
	var smnEvent smn.SMNTriggerEvent
	err := json.Unmarshal(payload, &smnEvent)
	if err != nil {
		slog.Error("unmarshal payload failed")
		return "invalid data", err
	}
	ctx.GetLogger().Logf("payload:%s", smnEvent.String())

	var c int = 1
	for _, record := range smnEvent.Record {

		var hd sharedmodule.HostingDetail
		if err := json.Unmarshal([]byte(record.Smn.Message), &hd); err != nil {
			slog.Error("unmarshal message failed")
			return "invalid data", err
		}

		slog.Info(fmt.Sprintf("hosting detail #%d:%+v", c, hd))
		c++

		var ttl int32 = 60

		_, err = h.Client.CreateRecordSet(&dnsModel.CreateRecordSetRequest{
			ZoneId: AppConfig.DnsZoneId,
			Body: &dnsModel.CreateRecordSetRequestBody{
				Name:    hd.SubDomain + "." + AppConfig.DnsZoneName,
				Type:    "A",
				Ttl:     &ttl,
				Records: []string{AppConfig.DnsRecordIp},
			},
		})

		if err != nil {
			slog.Error("create dns record failed", err.Error())
			return "create dns record failed", err
		}
	}

	slog.Info(fmt.Sprintf("Processed %d records", c-1))
	slog.Info("===========")
	return "ok", nil
}

func main() {

	loadConfig()

	cred, err := basic.NewCredentialsBuilder().WithAk(AppConfig.AccessKey).WithSk(AppConfig.SecretKey).SafeBuild()
	if err != nil {
		slog.Error("create huawei credential failed", err.Error())
		os.Exit(1)
	}

	dnsHcClient, err := dns.DnsClientBuilder().WithCredential(cred).WithRegion(dnsRegion.CN_NORTH_1).SafeBuild()
	if err != nil {
		slog.Error("create huawei dns client failed", err.Error())
		os.Exit(1)
	}

	client := dns.NewDnsClient(dnsHcClient)

	h := &handler{
		Client: client,
	}

	runtime.Register(h.SmnTrigger)
}

func stringPtr(s string) *string {
	return &s
}
