package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/merthan/otacon/internal/engine"
	"github.com/merthan/otacon/internal/engine/intelligence"
)

// ============================================================
// MICROSOFT TEAMS
// ============================================================

// TeamsConfig configures Teams notifications
type TeamsConfig struct {
	WebhookURL string
}

// TeamsChannel sends notifications via Teams webhook
type TeamsChannel struct {
	config TeamsConfig
	client *http.Client
}

func NewTeamsChannel(config TeamsConfig) *TeamsChannel {
	return &TeamsChannel{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *TeamsChannel) Name() string { return "teams" }

func (t *TeamsChannel) Send(ctx context.Context, payload engine.NotificationPayload) error {
	color := "00FF00"
	switch payload.Severity {
	case engine.SeverityCritical:
		color = "FF0000"
	case engine.SeverityWarning:
		color = "FFC107"
	}

	// Build facts from fields
	var facts []map[string]string
	for key, value := range payload.Fields {
		if value != "" {
			facts = append(facts, map[string]string{"name": key, "value": value})
		}
	}

	card := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"themeColor": color,
		"summary":    payload.Title,
		"sections": []map[string]interface{}{
			{
				"activityTitle": payload.Title,
				"activitySubtitle": fmt.Sprintf("Otacon Intelligence Platform — %s",
					payload.Timestamp.Format("15:04:05 MST")),
				"facts":   facts,
				"markdown": true,
				"text":    payload.Body,
			},
		},
	}

	body, _ := json.Marshal(card)
	req, err := http.NewRequestWithContext(ctx, "POST", t.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("teams webhook failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("teams webhook returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (t *TeamsChannel) SendDigest(ctx context.Context, digest *intelligence.Digest) error {
	payload := engine.NotificationPayload{
		Title:     fmt.Sprintf("📋 Otacon %s Digest — Grade: %s", digest.Type, digest.Summary.OverallGrade),
		Severity:  engine.SeverityInfo,
		Body:      intelligence.FormatDigestText(digest),
		Timestamp: digest.GeneratedAt,
	}
	return t.Send(ctx, payload)
}

// ============================================================
// EMAIL (SMTP)
// ============================================================

// EmailConfig configures email notifications
type EmailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
	UseTLS   bool
}

// EmailChannel sends notifications via SMTP
type EmailChannel struct {
	config EmailConfig
}

func NewEmailChannel(config EmailConfig) *EmailChannel {
	return &EmailChannel{config: config}
}

func (e *EmailChannel) Name() string { return "email" }

func (e *EmailChannel) Send(ctx context.Context, payload engine.NotificationPayload) error {
	subject := payload.Title
	body := e.buildEmailBody(payload)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		e.config.From,
		strings.Join(e.config.To, ", "),
		subject,
		body,
	)

	addr := fmt.Sprintf("%s:%d", e.config.Host, e.config.Port)
	var auth smtp.Auth
	if e.config.Username != "" {
		auth = smtp.PlainAuth("", e.config.Username, e.config.Password, e.config.Host)
	}

	err := smtp.SendMail(addr, auth, e.config.From, e.config.To, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	return nil
}

func (e *EmailChannel) SendDigest(ctx context.Context, digest *intelligence.Digest) error {
	payload := engine.NotificationPayload{
		Title:     fmt.Sprintf("Otacon %s Digest — Cluster: %s — Grade: %s", digest.Type, digest.ClusterName, digest.Summary.OverallGrade),
		Severity:  engine.SeverityInfo,
		Body:      intelligence.FormatDigestText(digest),
		Timestamp: digest.GeneratedAt,
	}
	return e.Send(ctx, payload)
}

func (e *EmailChannel) buildEmailBody(payload engine.NotificationPayload) string {
	severityColor := "#4CAF50"
	switch payload.Severity {
	case engine.SeverityCritical:
		severityColor = "#F44336"
	case engine.SeverityWarning:
		severityColor = "#FF9800"
	}

	fieldsHTML := ""
	for key, value := range payload.Fields {
		if value != "" {
			fieldsHTML += fmt.Sprintf("<tr><td style='padding:4px 8px;font-weight:bold;'>%s</td><td style='padding:4px 8px;'>%s</td></tr>", key, value)
		}
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><style>body{font-family:Arial,sans-serif;margin:0;padding:20px;background:#f5f5f5;}
.card{background:white;border-radius:8px;overflow:hidden;max-width:600px;margin:0 auto;box-shadow:0 2px 4px rgba(0,0,0,0.1);}
.header{background:%s;color:white;padding:16px 20px;font-size:16px;font-weight:bold;}
.body{padding:20px;}
table{width:100%%;border-collapse:collapse;}
.footer{padding:12px 20px;background:#f9f9f9;color:#666;font-size:12px;}</style></head>
<body>
<div class="card">
<div class="header">%s</div>
<div class="body">
<p>%s</p>
<table>%s</table>
</div>
<div class="footer">Otacon Intelligence Platform — %s</div>
</div>
</body></html>`,
		severityColor, payload.Title,
		strings.ReplaceAll(payload.Body, "\n", "<br>"),
		fieldsHTML,
		payload.Timestamp.Format("2006-01-02 15:04:05 MST"))
}

// ============================================================
// GENERIC WEBHOOK
// ============================================================

// WebhookConfig configures generic webhook notifications
type WebhookConfig struct {
	URL     string
	Headers map[string]string
	Method  string // default POST
}

// WebhookChannel sends notifications via HTTP webhook
type WebhookChannel struct {
	config WebhookConfig
	client *http.Client
}

func NewWebhookChannel(config WebhookConfig) *WebhookChannel {
	if config.Method == "" {
		config.Method = "POST"
	}
	return &WebhookChannel{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WebhookChannel) Name() string { return "webhook" }

func (w *WebhookChannel) Send(ctx context.Context, payload engine.NotificationPayload) error {
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, w.config.Method, w.config.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range w.config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (w *WebhookChannel) SendDigest(ctx context.Context, digest *intelligence.Digest) error {
	body, _ := json.Marshal(digest)
	req, err := http.NewRequestWithContext(ctx, w.config.Method, w.config.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range w.config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook digest failed: %w", err)
	}
	defer resp.Body.Close()
	return nil
}
