package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/merthan/otacon/internal/engine"
	"github.com/merthan/otacon/internal/engine/intelligence"
)

// SlackConfig configures Slack notifications
type SlackConfig struct {
	WebhookURL string
	Channel    string // Override channel (optional)
	Username   string // Bot username (default: Otacon)
	IconEmoji  string // Bot icon (default: :robot_face:)
}

// SlackChannel sends notifications via Slack webhook
type SlackChannel struct {
	config SlackConfig
	client *http.Client
}

// NewSlackChannel creates a new Slack notification channel
func NewSlackChannel(config SlackConfig) *SlackChannel {
	if config.Username == "" {
		config.Username = "Otacon"
	}
	if config.IconEmoji == "" {
		config.IconEmoji = ":robot_face:"
	}

	return &SlackChannel{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SlackChannel) Name() string { return "slack" }

// Send sends a notification to Slack with rich formatting
func (s *SlackChannel) Send(ctx context.Context, payload engine.NotificationPayload) error {
	blocks := s.buildBlocks(payload)
	return s.sendWebhook(ctx, blocks)
}

// SendDigest sends a digest summary to Slack
func (s *SlackChannel) SendDigest(ctx context.Context, digest *intelligence.Digest) error {
	blocks := s.buildDigestBlocks(digest)
	return s.sendWebhook(ctx, blocks)
}

func (s *SlackChannel) buildBlocks(payload engine.NotificationPayload) map[string]interface{} {
	// Color based on severity
	color := "#36a64f" // green
	switch payload.Severity {
	case engine.SeverityCritical:
		color = "#e01e5a" // red
	case engine.SeverityWarning:
		color = "#ecb22e" // yellow
	}

	// Build fields
	var fields []map[string]interface{}
	for key, value := range payload.Fields {
		if value != "" {
			fields = append(fields, map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s:* %s", key, value),
			})
		}
	}

	blocks := []interface{}{
		map[string]interface{}{
			"type": "header",
			"text": map[string]interface{}{
				"type": "plain_text",
				"text": payload.Title,
			},
		},
		map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": payload.Body,
			},
		},
	}

	// Add fields section
	if len(fields) > 0 {
		blocks = append(blocks, map[string]interface{}{
			"type":   "section",
			"fields": fields,
		})
	}

	// Add enrichments
	for _, enrichment := range payload.Enrichments {
		switch enrichment.Type {
		case "logs":
			blocks = append(blocks, map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*%s:*\n```%s```", enrichment.Title, truncateStr(enrichment.Content, 500)),
				},
			})
		case "link":
			blocks = append(blocks, map[string]interface{}{
				"type": "context",
				"elements": []map[string]interface{}{
					{
						"type": "mrkdwn",
						"text": fmt.Sprintf("📊 <%s|%s>", enrichment.Content, enrichment.Title),
					},
				},
			})
		}
	}

	// Add timestamp context
	blocks = append(blocks, map[string]interface{}{
		"type": "context",
		"elements": []map[string]interface{}{
			{
				"type": "mrkdwn",
				"text": fmt.Sprintf("_Otacon Intelligence Platform — %s_", payload.Timestamp.Format("15:04:05 MST")),
			},
		},
	})

	msg := map[string]interface{}{
		"username":   s.config.Username,
		"icon_emoji": s.config.IconEmoji,
		"blocks":     blocks,
		"attachments": []map[string]interface{}{
			{"color": color, "blocks": []interface{}{}},
		},
	}

	if s.config.Channel != "" {
		msg["channel"] = s.config.Channel
	}

	return msg
}

func (s *SlackChannel) buildDigestBlocks(digest *intelligence.Digest) map[string]interface{} {
	summary := digest.Summary

	gradeEmoji := "📊"
	switch {
	case summary.OverallScore >= 90:
		gradeEmoji = "🟢"
	case summary.OverallScore >= 70:
		gradeEmoji = "🟡"
	default:
		gradeEmoji = "🔴"
	}

	trendStr := ""
	if summary.ScoreChange != 0 {
		direction := "↑"
		if summary.ScoreChange < 0 {
			direction = "↓"
		}
		trendStr = fmt.Sprintf(" (%s%.1f)", direction, summary.ScoreChange)
	}

	blocks := []interface{}{
		map[string]interface{}{
			"type": "header",
			"text": map[string]interface{}{
				"type": "plain_text",
				"text": fmt.Sprintf("📋 Otacon %s Digest", capitalizeFirst(string(digest.Type))),
			},
		},
		map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Cluster:* %s\n*Period:* %s to %s",
					digest.ClusterName,
					digest.PeriodStart.Format("Jan 02 15:04"),
					digest.PeriodEnd.Format("Jan 02 15:04")),
			},
		},
		map[string]interface{}{
			"type": "section",
			"fields": []map[string]interface{}{
				{"type": "mrkdwn", "text": fmt.Sprintf("%s *Grade:* %s (%.0f/100)%s", gradeEmoji, summary.OverallGrade, summary.OverallScore, trendStr)},
				{"type": "mrkdwn", "text": fmt.Sprintf("📈 *Status:* %s", summary.HealthStatus)},
				{"type": "mrkdwn", "text": fmt.Sprintf("🔴 *Critical:* %d events", summary.CriticalEvents)},
				{"type": "mrkdwn", "text": fmt.Sprintf("🟡 *Warning:* %d events", summary.WarningEvents)},
				{"type": "mrkdwn", "text": fmt.Sprintf("🚨 *Incidents:* %d", summary.IncidentCount)},
				{"type": "mrkdwn", "text": fmt.Sprintf("📊 *Total Events:* %d", summary.TotalEvents)},
			},
		},
	}

	// Add top incidents
	if len(digest.Incidents) > 0 {
		incText := "*Top Incidents:*\n"
		for i, inc := range digest.Incidents {
			if i >= 3 {
				incText += fmt.Sprintf("_...and %d more_\n", len(digest.Incidents)-3)
				break
			}
			incText += fmt.Sprintf("• %s %s (%d events)\n", inc.Severity.Icon(), inc.Title, len(inc.Events))
		}
		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": incText,
			},
		})
	}

	blocks = append(blocks, map[string]interface{}{
		"type": "context",
		"elements": []map[string]interface{}{
			{
				"type": "mrkdwn",
				"text": fmt.Sprintf("_Generated by Otacon at %s_", digest.GeneratedAt.Format("2006-01-02 15:04:05 MST")),
			},
		},
	})

	return map[string]interface{}{
		"username":   s.config.Username,
		"icon_emoji": s.config.IconEmoji,
		"blocks":     blocks,
	}
}

func (s *SlackChannel) sendWebhook(ctx context.Context, message map[string]interface{}) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack webhook returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
