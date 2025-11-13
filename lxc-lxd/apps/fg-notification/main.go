package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/smtp"
	"os"
	"sharedmodule"
	"strconv"

	"github.com/joho/godotenv"
	"huaweicloud.com/go-runtime/events/smn"
	fgcontext "huaweicloud.com/go-runtime/go-api/context"
	"huaweicloud.com/go-runtime/pkg/runtime"
)

type Config struct {
	Local bool
	Smtp  ConfigSmtp
}

type ConfigSmtp struct {
	Host       string
	Port       int
	Email      string
	Password   string
	SenderName string
}

var AppConfig Config

func loadConfig() {
	if err := godotenv.Load(); err != nil {
		fmt.Printf("failed to load .env file: %s\n", err.Error())
	}

	AppConfig.Local, _ = strconv.ParseBool(getEnv("LOCAL", "false"))
	AppConfig.Smtp.Host = getEnv("SMTP_HOST", "smtp.example.com")
	AppConfig.Smtp.Port = 587
	AppConfig.Smtp.Email = getEnv("SMTP_EMAIL", "dianraha11@gmail.com")
	AppConfig.Smtp.Password = getEnv("SMTP_PASSWORD", "password")
	AppConfig.Smtp.SenderName = getEnv("SMTP_SENDER_NAME", "Demo Huawei <dianraha11@gmail.com>")
}

func getEnv(key, defaultValue string) string {
	if value, exist := os.LookupEnv(key); exist {
		return value
	}
	return defaultValue
}

func sendEmail(to, subject, message string) error {
	body := fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\nMIME-Version: 1.0\nContent-Type: text/html; charset=UTF-8\n\n%s", AppConfig.Smtp.SenderName, to, subject, message)

	auth := smtp.PlainAuth("", AppConfig.Smtp.Email, AppConfig.Smtp.Password, AppConfig.Smtp.Host)
	smtpAddr := fmt.Sprintf("%s:%d", AppConfig.Smtp.Host, AppConfig.Smtp.Port)

	return smtp.SendMail(smtpAddr, auth, AppConfig.Smtp.Email, []string{to}, []byte(body))
}

func SmnTrigger(payload []byte, ctx fgcontext.RuntimeContext) (interface{}, error) {
	var smnEvent smn.SMNTriggerEvent
	err := json.Unmarshal(payload, &smnEvent)
	if err != nil {
		slog.Info("unmarshal payload failed")
		return "invalid data", err
	}
	ctx.GetLogger().Logf("payload:%s", smnEvent.String())

	var c int = 1
	for _, record := range smnEvent.Record {
		var n sharedmodule.Notification
		err = json.Unmarshal([]byte(record.Smn.Message), &n)
		if err != nil {
			slog.Info("unmarshal notification failed")
			return "invalid data", err
		}

		slog.Info(fmt.Sprintf("notification #%d:%+v", c, n))
		c++

		if err := sendEmail(n.Receiver, n.Subject, n.Message); err != nil {
			slog.Info(fmt.Sprintf("failed to send email to %s: %v", n.Receiver, err))
		}
	}

	slog.Info(fmt.Sprintf("Processed %d records", c-1))
	slog.Info("===========")
	return "ok", nil
}

func main() {

	loadConfig()

	if AppConfig.Local {
		slog.Info("running in local mode")
		if err := sendEmail("pipelinekirimuang@gmail.com", "Test Email", "This is a test email from local run"); err != nil {
			slog.Error("failed to send test email", err.Error())
		}
	} else {
		runtime.Register(SmnTrigger)
	}
}
