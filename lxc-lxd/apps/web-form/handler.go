package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"regexp"
	"sharedmodule"
	"text/template"

	smn "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/smn/v2"
	smnModel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/smn/v2/model"
)

type handler struct {
	tmpl      *template.Template
	smnClient *smn.SmnClient
}

func (h *handler) indexHandler(w http.ResponseWriter, r *http.Request) {

	data := map[string]interface{}{
		"Values": map[string]string{
			"SubdomainName":  "",
			"WordPressTheme": "",
			"Email":          "",
		},
	}

	if err := h.tmpl.ExecuteTemplate(w, "form.html", data); err != nil {
		slog.Error("failed to execute template", slog.String("error", err.Error()))
		http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (h *handler) submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// validate form inputs
	var formErr []string

	subdomainName := r.FormValue("subdomain_name")
	wordpressTheme := r.FormValue("wordpress_theme")
	email := r.FormValue("email")

	if subdomainName == "" {
		formErr = append(formErr, "Subdomain name is required")
	}

	subdomainRegex := regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	if !subdomainRegex.MatchString(subdomainName) {
		formErr = append(formErr, "Invalid subdomain name: Invalid subdomain format. Only lowercase letters, numbers, and hyphens are allowed, and it cannot start or end with a hyphen.")
	}

	if wordpressTheme == "" {
		formErr = append(formErr, "WordPress theme is required")
	}

	// check that wordpressTheme is in the allowed list
	themeList := []sharedmodule.ThemeSelection{
		sharedmodule.ThemeTwentyTwentyFour,
		sharedmodule.ThemeTwentyTwentyTwo,
		sharedmodule.ThemeTwentyTwentyFive,
	}

	var themeMatch bool
	for _, theme := range themeList {
		if wordpressTheme == string(theme) {
			themeMatch = true
			break
		}
	}
	if !themeMatch {
		formErr = append(formErr, "Invalid WordPress theme selection")
	}

	if email == "" {
		formErr = append(formErr, "Email is required")
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		formErr = append(formErr, "Invalid email format")
	}

	if len(formErr) > 0 {
		// re-render the form with error messages + pre-filled values
		w.WriteHeader(http.StatusBadRequest)
		if err := h.tmpl.ExecuteTemplate(w, "form.html", map[string]interface{}{
			"Errors": formErr,
			"Values": map[string]string{
				"SubdomainName":  subdomainName,
				"WordPressTheme": wordpressTheme,
				"Email":          email,
			},
		}); err != nil {
			slog.Error("failed to execute template", slog.String("error", err.Error()))
			http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// reset formErr
	formErr = []string{}

	// send notification to SMN
	dataNotification := sharedmodule.Notification{
		Type:     sharedmodule.NotificationTypeEmail,
		Subject:  fmt.Sprintf("Your hosting for %s is being processed", subdomainName),
		Message:  fmt.Sprintf("This detail your hosting.<br><br> Domain: %s.%s<br> Theme: %s<br> Contact email: %s", subdomainName, appConfig.RootDomainName, wordpressTheme, email),
		Receiver: email,
	}

	if err := h.publishMessage(dataNotification, appConfig.SmnTopicNotificationUrn); err != nil {
		slog.Error("failed to publish notification message", slog.String("error", err.Error()))
		formErr = append(formErr, "Failed to send notification: "+err.Error())
		return
	}

	// send hosting detail to SMN
	dataHosting := sharedmodule.HostingDetail{
		SubDomain: subdomainName,
		Theme:     sharedmodule.ThemeSelection(wordpressTheme),
		Email:     email,
	}

	if err := h.publishMessage(dataHosting, appConfig.SmnTopicHostingUrn); err != nil {
		slog.Error("failed to publish hosting message", slog.String("error", err.Error()))
		formErr = append(formErr, "Failed to process hosting: "+err.Error())
		return
	}

	if len(formErr) > 0 {
		// re-render the form with error messages + pre-filled values
		w.WriteHeader(http.StatusBadRequest)
		if err := h.tmpl.ExecuteTemplate(w, "form.html", map[string]interface{}{
			"Errors": formErr,
			"Values": map[string]string{
				"SubdomainName":  subdomainName,
				"WordPressTheme": wordpressTheme,
				"Email":          email,
			},
		}); err != nil {
			slog.Error("failed to execute template", slog.String("error", err.Error()))
			http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if err := h.tmpl.ExecuteTemplate(w, "form.html", map[string]interface{}{
		"Success": "Form submitted successfully!",
		"Values": map[string]string{
			"SubdomainName":  "",
			"WordPressTheme": "",
			"Email":          "",
		},
	}); err != nil {
		slog.Error("failed to execute template", slog.String("error", err.Error()))
		http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (h *handler) publishMessage(data interface{}, topic string) error {

	switch v := data.(type) {
	case sharedmodule.HostingDetail:
	case sharedmodule.Notification:
	default:
		return fmt.Errorf("unknown data type: %s", reflect.TypeOf(v).String())
	}

	jsonMessage, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	m := string(jsonMessage)
	smnRequest := &smnModel.PublishMessageRequest{
		TopicUrn: topic,
		Body: &smnModel.PublishMessageRequestBody{
			Message: &m,
		},
	}

	_, err = h.smnClient.PublishMessage(smnRequest)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}
