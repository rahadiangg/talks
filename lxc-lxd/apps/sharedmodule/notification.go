package sharedmodule

type NotificationType string

const (
	NotificationTypeEmail NotificationType = "email"
	NotificationTypeSMS   NotificationType = "telegram"
)

type Notification struct {
	Type     NotificationType `json:"type"`
	Subject  string           `json:"subject"`
	Message  string           `json:"message"`
	Receiver string           `json:"receiver"`
}
