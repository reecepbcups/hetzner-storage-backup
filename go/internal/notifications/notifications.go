// Package notifications ports src/notifications.py: posts a Discord embed webhook.
package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Field is one embed field: value[0]=text, value[1]=inline.
type Field struct {
	Text   string
	Inline bool
}

type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type embedThumbnail struct {
	URL string `json:"url"`
}

type embedFooter struct {
	Text string `json:"text"`
}

type embed struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Color       int64           `json:"color"`
	Thumbnail   embedThumbnail  `json:"thumbnail"`
	Footer      embedFooter     `json:"footer"`
	Timestamp   string          `json:"timestamp"`
	Fields      []embedField    `json:"fields"`
}

type webhookPayload struct {
	Embeds []embed `json:"embeds"`
}

// DiscordNotification posts an embed to the given webhook url, matching
// discord_notification() in notifications.py. values keys become field
// names in insertion order; color is a hex string like "ff0000".
func DiscordNotification(enabled bool, url, title, description, color string, values map[string]Field, valueOrder []string, imageLink, footerText string) bool {
	if !enabled {
		fmt.Println("Webhook notifs are disabled in config")
		return false
	}

	var colorInt int64
	fmt.Sscanf(color, "%x", &colorInt)

	fields := make([]embedField, 0, len(valueOrder))
	for _, k := range valueOrder {
		v := values[k]
		fields = append(fields, embedField{Name: k, Value: v.Text, Inline: v.Inline})
	}

	payload := webhookPayload{
		Embeds: []embed{
			{
				Title:       title,
				Description: description,
				Color:       colorInt,
				Thumbnail:   embedThumbnail{URL: imageLink},
				Footer:      embedFooter{Text: footerText},
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
				Fields:      fields,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error building discord payload:", err)
		return false
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Println("Error sending discord webhook:", err)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
