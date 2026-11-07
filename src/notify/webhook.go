package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const webhookRequestTimeout = 10 * time.Second

var webhookRetryDelays = [...]time.Duration{
	time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	time.Hour,
	6 * time.Hour,
	24 * time.Hour,
}

type webhookConfigurationError struct {
	message string
}

func (e *webhookConfigurationError) Error() string { return e.message }

func newWebhookConfigurationError(message string) error {
	return &webhookConfigurationError{message: message}
}

// WebhookTarget identifies one configured destination. Secret is the
// per-webhook signing secret stored beside its URL in server.yml.
type WebhookTarget struct {
	Role      string
	Transport string
	URL       string
	Secret    string
}

// WebhookMessage carries the event fields shared by the built-in adapters.
type WebhookMessage struct {
	Event          string
	Subject        string
	Body           string
	Severity       string
	Timestamp      time.Time
	ProjectName    string
	ProjectVersion string
	AppURL         string
	TrackingID     string
}

// webhookPayload is the JSON shape sent by the generic adapter.
type webhookPayload struct {
	Role           string    `json:"role"`
	Event          string    `json:"event"`
	Subject        string    `json:"subject"`
	Body           string    `json:"body"`
	Severity       string    `json:"severity"`
	Timestamp      time.Time `json:"timestamp"`
	ProjectName    string    `json:"project_name"`
	ProjectVersion string    `json:"project_version"`
	AppURL         string    `json:"app_url"`
	TrackingID     string    `json:"tracking_id,omitempty"`
}

// DeliverWebhook sends one webhook attempt and returns the generated UUIDv7
// delivery ID. The caller may retain that ID when it schedules a retry.
func DeliverWebhook(ctx context.Context, target WebhookTarget, message WebhookMessage) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	deliveryID, err := uuid.NewV7()
	if err != nil {
		return "", errors.New("notify: could not create webhook delivery ID")
	}
	if err := deliverWebhookAttempt(ctx, target, message, deliveryID.String()); err != nil {
		return deliveryID.String(), err
	}
	return deliveryID.String(), nil
}

// DeliverWebhookWithRetry sends immediately, then retries non-2xx and
// transport failures using the PART 12 schedule. Retries share one UUIDv7
// delivery ID so receivers can deduplicate them. onPermanentFailure is called
// once after all seven attempts fail; callers can log the notify.webhook_failed
// event through their rate-limited event logger.
func DeliverWebhookWithRetry(ctx context.Context, target WebhookTarget, message WebhookMessage, onPermanentFailure func(error)) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	deliveryID, err := uuid.NewV7()
	if err != nil {
		return "", errors.New("notify: could not create webhook delivery ID")
	}
	for attempt := 0; ; attempt++ {
		err = deliverWebhookAttempt(ctx, target, message, deliveryID.String())
		if err == nil {
			return deliveryID.String(), nil
		}
		var configurationErr *webhookConfigurationError
		if errors.As(err, &configurationErr) {
			if onPermanentFailure != nil {
				onPermanentFailure(err)
			}
			return deliveryID.String(), err
		}
		if attempt == len(webhookRetryDelays) {
			if onPermanentFailure != nil {
				onPermanentFailure(err)
			}
			return deliveryID.String(), err
		}
		timer := time.NewTimer(webhookRetryDelays[attempt])
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return deliveryID.String(), ctx.Err()
		case <-timer.C:
		}
	}
}

func deliverWebhookAttempt(ctx context.Context, target WebhookTarget, message WebhookMessage, deliveryID string) error {
	transport := strings.ToLower(strings.TrimSpace(target.Transport))
	if target.Secret == "" {
		return newWebhookConfigurationError("notify: webhook signing secret is not configured")
	}
	if strings.TrimSpace(message.Event) == "" {
		return newWebhookConfigurationError("notify: webhook event is required")
	}
	if strings.TrimSpace(message.ProjectName) == "" || strings.TrimSpace(message.ProjectVersion) == "" || strings.TrimSpace(message.AppURL) == "" {
		return newWebhookConfigurationError("notify: project name, version, and app URL are required for webhook delivery")
	}
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now().UTC()
	}
	endpoint, body, contentType, err := buildWebhookRequest(target, message)
	if err != nil {
		return newWebhookConfigurationError(err.Error())
	}
	requestCtx, cancel := context.WithTimeout(ctx, webhookRequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return newWebhookConfigurationError(fmt.Sprintf("notify: invalid %s webhook request", transport))
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	request.Header.Set("X-Webhook-Signature", webhookSignature(target.Secret, body))
	request.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	request.Header.Set("X-Webhook-ID", deliveryID)
	request.Header.Set("X-Webhook-Event", message.Event)
	request.Header.Set("User-Agent", webhookUserAgent(message))

	client := &http.Client{
		Timeout: webhookRequestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("notify: %s webhook request failed", transport)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8<<10))
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("notify: %s webhook returned HTTP %d", transport, response.StatusCode)
	}
	return nil
}

func buildWebhookRequest(target WebhookTarget, message WebhookMessage) (endpoint string, body []byte, contentType string, err error) {
	transport := strings.ToLower(strings.TrimSpace(target.Transport))
	parsed, err := parseWebhookURL(target.URL)
	if err != nil {
		return "", nil, "", err
	}
	var payload any
	switch transport {
	case "telegram":
		query := parsed.Query()
		if query.Get("chat_id") == "" {
			return "", nil, "", errors.New("notify: Telegram webhook URL must include chat_id")
		}
		query.Set("text", webhookText(message))
		parsed.RawQuery = query.Encode()
		return parsed.String(), nil, "", nil
	case "discord":
		payload = struct {
			Content  string `json:"content"`
			Username string `json:"username"`
		}{Content: webhookText(message), Username: message.ProjectName}
	case "slack", "mattermost":
		payload = struct {
			Text string `json:"text"`
		}{Text: webhookText(message)}
	case "pushover":
		query := parsed.Query()
		user, token := query.Get("user"), query.Get("token")
		if user == "" || token == "" {
			return "", nil, "", errors.New("notify: Pushover webhook URL must include user and token")
		}
		form := url.Values{
			"user":    {user},
			"token":   {token},
			"title":   {message.Subject},
			"message": {webhookText(message)},
		}
		if message.Severity != "" {
			form.Set("priority", pushoverPriority(message.Severity))
		}
		parsed.RawQuery = ""
		parsed.ForceQuery = false
		return parsed.String(), []byte(form.Encode()), "application/x-www-form-urlencoded", nil
	case "gotify":
		query := parsed.Query()
		token := query.Get("token")
		if token == "" {
			return "", nil, "", errors.New("notify: Gotify webhook URL must include token")
		}
		path := strings.TrimRight(parsed.Path, "/")
		if !strings.HasSuffix(path, "/message") {
			path += "/message"
		}
		parsed.Path = path
		parsed.RawPath = ""
		query.Set("token", token)
		parsed.RawQuery = query.Encode()
		payload = struct {
			Title    string `json:"title"`
			Message  string `json:"message"`
			Priority int    `json:"priority"`
		}{Title: message.Subject, Message: webhookText(message), Priority: gotifyPriority(message.Severity)}
	case "generic":
		payload = webhookPayload{
			Role: target.Role, Event: message.Event, Subject: message.Subject,
			Body: message.Body, Severity: message.Severity, Timestamp: message.Timestamp,
			ProjectName: message.ProjectName, ProjectVersion: message.ProjectVersion,
			AppURL: message.AppURL, TrackingID: message.TrackingID,
		}
	default:
		return "", nil, "", fmt.Errorf("notify: unsupported webhook transport %q", transport)
	}
	body, err = json.Marshal(payload)
	if err != nil {
		return "", nil, "", errors.New("notify: could not encode webhook payload")
	}
	return parsed.String(), body, "application/json", nil
}

func parseWebhookURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New("notify: webhook URL must be a valid HTTPS URL without userinfo or fragment")
	}
	return parsed, nil
}

func webhookSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func webhookUserAgent(message WebhookMessage) string {
	name := strings.TrimSpace(message.ProjectName)
	version := strings.TrimSpace(message.ProjectVersion)
	appURL := strings.TrimSpace(message.AppURL)
	return name + "/" + version + " (+" + appURL + ")"
}

func webhookText(message WebhookMessage) string {
	if message.Body == "" {
		return message.Subject
	}
	if message.Subject == "" {
		return message.Body
	}
	return message.Subject + "\n" + message.Body
}

func pushoverPriority(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "error":
		return "1"
	case "warning", "warn":
		return "0"
	default:
		return "-1"
	}
}

func gotifyPriority(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 10
	case "error":
		return 8
	case "warning", "warn":
		return 5
	default:
		return 1
	}
}
