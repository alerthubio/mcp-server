package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultBaseURL = "https://alerthub.io"
	maxTitleBytes  = 50
	maxBodyChars   = 250
)

// Client wraps AlertHub API credentials, HTTP client, and a local anti-spam rate limiter.
type Client struct {
	baseURL    string
	userID     string
	apiKey     string
	version    string
	httpClient *http.Client

	mu         sync.Mutex
	timestamps []time.Time
}

// ResponseModel is a limiter interface for the decodeResponse generic function
type ResponseModel interface {
	ProfileSummary | EventSummary
}

func NewClientFromEnv(ctx context.Context, version string) (*Client, error) {
	userID := strings.TrimSpace(os.Getenv("ALERTHUB_USER_ID"))
	apiKey := strings.TrimSpace(os.Getenv("ALERTHUB_API_KEY"))
	if userID == "" || apiKey == "" {
		return nil, errors.New("ALERTHUB_USER_ID and ALERTHUB_API_KEY environment variables are required")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("ALERTHUB_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &Client{
		baseURL: baseURL,
		userID:  userID,
		apiKey:  apiKey,
		version: version,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 5 * time.Second,
				IdleConnTimeout:       60 * time.Second,
				MaxIdleConns:          300,
				MaxIdleConnsPerHost:   100,
			},
		},
	}, nil
}

// allowDispatch enforces a burst limit of 5 alerts per 60 seconds to prevent runaway agent loops.
func (c *Client) allowDispatch() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)
	valid := c.timestamps[:0]
	for _, ts := range c.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	c.timestamps = valid

	if len(c.timestamps) >= 5 {
		return errors.New("rate limit exceeded: max 5 AlertHub dispatches per minute from MCP")
	}
	c.timestamps = append(c.timestamps, now)
	return nil
}

// postJSON makes a POST request to the API server. Receives a context, path for the request and the payload.
// Returns status code,response body as string or an error
func (c *Client) postJSON(ctx context.Context, path string, payload any) (int, string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("X-API-KEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "alerthub-mcp/"+c.version)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return resp.StatusCode, string(respBytes), nil
}

// getJSON makes a GET request to the API server.  Receives a context and a path for the request.
// Returns status code,response body as an []bytes or an error
func (c *Client) getJSON(ctx context.Context, path string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("X-API-KEY", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "alerthub-mcp/"+c.version)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	// Cap response read size (64 KB) to prevent oversized payloads from exhausting memory
	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, respBytes, nil
}

// clampTitle prevents the title from surpassing the max limit of characters
func clampTitle(text string) (string, bool) {
	text = strings.TrimSpace(text)
	encoded := []byte(text)
	if len(encoded) <= maxTitleBytes {
		return text, false
	}
	trimmed := strings.TrimSpace(string(encoded[:maxTitleBytes-3]))
	return trimmed + "...", true
}

// truncateBody caps the event body to the max number of characters
func truncateBody(text string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) <= maxBodyChars {
		return text
	}
	return string(runes[:maxBodyChars-3]) + "..."
}

// decodeResponse supports both a paginated envelope ({"data": [...]}) and a direct JSON array ([...]).
func decodeResponse[T ResponseModel](raw []byte) ([]T, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}

	if trimmed[0] == '[' {
		var items []T
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, fmt.Errorf("decode array response: %w", err)
		}
		return items, nil
	}

	var envelope struct {
		Data []T `json:"data"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil, fmt.Errorf("decode paginated response: %w", err)
	}
	return envelope.Data, nil
}

// enforceSingleTypeArrays converts ["null", "array"] slice schemas into single-type "array"
// schemas so tools pass strict single-type portability checks (e.g. OpenAPI / Gemini).
func enforceSingleTypeArrays(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	if len(s.Types) == 2 &&
		((s.Types[0] == "null" && s.Types[1] == "array") ||
			(s.Types[0] == "array" && s.Types[1] == "null")) {
		s.Types = nil
		s.Type = "array"
	}
	for _, prop := range s.Properties {
		enforceSingleTypeArrays(prop)
	}
	enforceSingleTypeArrays(s.Items)
}

// --- Tool 1: send_alert ---

type SendAlertInput struct {
	Title    string `json:"title" jsonschema:"Short alert headline (max 50 characters; auto-truncated if longer)"`
	Body     string `json:"body" jsonschema:"Detailed alert message explaining what happened or requires attention"`
	Profile  string `json:"profile,omitempty" jsonschema:"Optional AlertHub notification profile id for channel routing. List of profile ids available using list_profiles tool. (default=Mail and Push notification)"`
	Status   string `json:"status,omitempty" jsonschema:"Optional event status"`
	Severity string `json:"severity,omitempty" jsonschema:"Optional severity status"`
	Priority string `json:"priority,omitempty" jsonschema:"Optional alert priority: low, normal, or high"`
}

type ToolResultOutput struct {
	Success    bool   `json:"success"`
	HTTPStatus int    `json:"http_status"`
	Message    string `json:"message"`
}

func (c *Client) SendAlert(ctx context.Context, _ *mcp.CallToolRequest, in SendAlertInput) (*mcp.CallToolResult, ToolResultOutput, error) {
	if err := c.allowDispatch(); err != nil {
		return nil, ToolResultOutput{}, err
	}

	rawTitle := strings.TrimSpace(in.Title)
	if rawTitle == "" {
		return nil, ToolResultOutput{}, errors.New("title is required")
	}
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return nil, ToolResultOutput{}, errors.New("body is required")
	}

	title, truncated := clampTitle(rawTitle)
	if truncated {
		body = rawTitle + "\n\n" + body
	}

	status := strings.ToLower(strings.TrimSpace(in.Status))
	//if status == "" {
	//	status = "info"
	//}
	priority := strings.ToLower(strings.TrimSpace(in.Priority))
	//if priority == "" {
	//	priority = "normal"
	//}
	severity := strings.ToLower(strings.TrimSpace(in.Severity))

	payload := map[string]string{
		"title":    title,
		"body":     body,
		"status":   status,
		"priority": priority,
		"severity": severity,
		"source":   "mcp-server",
	}
	if p := strings.TrimSpace(in.Profile); p != "" {
		payload["profile"] = p
	}

	code, respBody, err := c.postJSON(ctx, "/api/event/"+c.userID, payload)
	if err != nil {
		return nil, ToolResultOutput{}, err
	}
	if code < 200 || code >= 300 {
		return nil, ToolResultOutput{}, fmt.Errorf("AlertHub API error (HTTP %d): %s", code, respBody)
	}

	return nil, ToolResultOutput{
		Success:    true,
		HTTPStatus: code,
		Message:    fmt.Sprintf("Dispatched alert %q", title),
	}, nil
}

// --- Tool 2: schedule_alert ---

type ScheduleAlertInput struct {
	Title     string `json:"title" jsonschema:"Short reminder headline (3 to 50 characters)"`
	Body      string `json:"body" jsonschema:"Reminder details (2 to 250 characters)"`
	StartTime string `json:"start_time" jsonschema:"RFC3339 UTC timestamp at least 20 minutes in the future (e.g. 2027-05-01T09:00:00Z)"`
	Profile   string `json:"profile,omitempty" jsonschema:"Optional AlertHub notification profile id (default: default)"`
	Frequency string `json:"frequency,omitempty" jsonschema:"Optional recurrence frequency: DAILY, WEEKLY, MONTHLY, or YEARLY"`
	Interval  int    `json:"interval,omitempty" jsonschema:"Optional recurrence interval >= 1 (default: 1)"`
}

func (c *Client) ScheduleAlert(ctx context.Context, _ *mcp.CallToolRequest, in ScheduleAlertInput) (*mcp.CallToolResult, ToolResultOutput, error) {
	if err := c.allowDispatch(); err != nil {
		return nil, ToolResultOutput{}, err
	}

	title, _ := clampTitle(in.Title)
	if len(title) < 3 {
		return nil, ToolResultOutput{}, errors.New("title must be at least 3 characters")
	}

	body := truncateBody(in.Body)
	if len(body) < 2 {
		return nil, ToolResultOutput{}, errors.New("body must be at least 2 characters")
	}

	startTime, err := time.Parse(time.RFC3339, strings.TrimSpace(in.StartTime))
	if err != nil {
		return nil, ToolResultOutput{}, errors.New("start_time must be a valid RFC3339 timestamp (e.g. 2027-05-01T09:00:00Z)")
	}
	if startTime.UTC().Before(time.Now().UTC().Add(20 * time.Minute)) {
		return nil, ToolResultOutput{}, errors.New("start_time must be at least 20 minutes in the future")
	}

	payload := map[string]any{
		"title":      title,
		"body":       body,
		"start_time": startTime.UTC().Format(time.RFC3339),
	}
	if p := strings.TrimSpace(in.Profile); p != "" {
		payload["profile"] = p
	}
	if freq := strings.ToUpper(strings.TrimSpace(in.Frequency)); freq != "" {
		interval := in.Interval
		if interval < 1 {
			interval = 1
		}
		payload["recurrence"] = map[string]any{
			"frequency": freq,
			"interval":  interval,
			"end_type":  "forever",
		}
	}

	code, respBody, err := c.postJSON(ctx, "/api/schedule/"+c.userID, payload)
	if err != nil {
		return nil, ToolResultOutput{}, err
	}
	if code < 200 || code >= 300 {
		return nil, ToolResultOutput{}, fmt.Errorf("AlertHub API error (HTTP %d): %s", code, respBody)
	}

	return nil, ToolResultOutput{
		Success:    true,
		HTTPStatus: code,
		Message:    fmt.Sprintf("Scheduled alert %q for %s", title, startTime.UTC().Format(time.RFC3339)),
	}, nil
}

// --- Tool 3: list_recent_events ---

type ListRecentEventsInput struct {
	Limit  int    `json:"limit,omitempty" jsonschema:"Number of recent events to return (1 to 20, default: 10)"`
	Status string `json:"status,omitempty" jsonschema:"Optional filter by event status (e.g. info, success, complete, error)"`
	Source string `json:"source,omitempty" jsonschema:"Optional filter by event source identifier (e.g. github-actions, ci-deploy)"`
}

type EventSummary struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Status   string `json:"status,omitempty"`
	Priority string `json:"priority,omitempty"`
	Source   string `json:"source,omitempty"`
	Profile  string `json:"profile,omitempty"`
	Time     string `json:"time"`
}

type ListRecentEventsOutput struct {
	Count  int            `json:"count"`
	Events []EventSummary `json:"events"`
}

func (c *Client) ListRecentEvents(ctx context.Context, _ *mcp.CallToolRequest, in ListRecentEventsInput) (*mcp.CallToolResult, ListRecentEventsOutput, error) {
	if err := c.allowDispatch(); err != nil {
		return nil, ListRecentEventsOutput{}, err
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	statusFilter := strings.ToLower(strings.TrimSpace(in.Status))
	sourceFilter := strings.TrimSpace(in.Source)

	// Fetch up to 20 recent items when filtering so we still return up to `limit` matches
	fetchCount := limit
	if statusFilter != "" || sourceFilter != "" {
		fetchCount = 20
	}

	path := fmt.Sprintf("/api/events?index=1&count=%d", fetchCount)
	code, respBytes, err := c.getJSON(ctx, path)
	if err != nil {
		return nil, ListRecentEventsOutput{}, err
	}
	if code < 200 || code >= 300 {
		return nil, ListRecentEventsOutput{}, fmt.Errorf("AlertHub API error (HTTP %d): %s", code, string(respBytes))
	}

	rawEvents, err := decodeResponse[EventSummary](respBytes)
	if err != nil {
		return nil, ListRecentEventsOutput{}, err
	}

	filtered := make([]EventSummary, 0, len(rawEvents))
	for _, ev := range rawEvents {
		if statusFilter != "" && !strings.EqualFold(strings.TrimSpace(ev.Status), statusFilter) {
			continue
		}
		if sourceFilter != "" && !strings.EqualFold(strings.TrimSpace(ev.Source), sourceFilter) {
			continue
		}

		ev.Body = truncateBody(ev.Body)
		filtered = append(filtered, ev)
		if len(filtered) >= limit {
			break
		}
	}

	return nil, ListRecentEventsOutput{
		Count:  len(filtered),
		Events: filtered,
	}, nil
}

// --- Tool 4: list_profile ---

type ListProfilesInput struct {
	Limit       int  `json:"limit,omitempty" jsonschema:"Number of recent events to return (1 to 20, default: 10)"`
	IncDefaults bool `json:"defaults" jsonschema:"Optional filter to include or remove default profiles from the list (default=true)"`
}

type ProfileSummary struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	UserID          string    `json:"userID"`
	Mail            bool      `json:"email"`
	MobilePush      bool      `json:"push"`
	WhatsAppMessage bool      `json:"whatsapp"`
	Telegram        bool      `json:"telegram"`
	Slack           bool      `json:"slack"`
	Discord         bool      `json:"discord"`
	WebHook         string    `json:"webhook_id,omitempty"`
	CreatedAt       time.Time `json:"created"`
}

type ListProfilesOutput struct {
	Count    int              `json:"count"`
	Profiles []ProfileSummary `json:"profile"`
}

func (c *Client) ListProfiles(ctx context.Context, _ *mcp.CallToolRequest, in ListProfilesInput) (*mcp.CallToolResult, ListProfilesOutput, error) {
	var defaultsFilter = true
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	defaultsFilter = in.IncDefaults

	// Fetch up to 20 recent items when filtering so we still return up to `limit` matches
	fetchCount := limit
	if !defaultsFilter {
		fetchCount = 20
	}

	path := fmt.Sprintf("/api/profiles?index=1&count=%d", fetchCount)
	code, respBytes, err := c.getJSON(ctx, path)
	if err != nil {
		return nil, ListProfilesOutput{}, err
	}
	if code < 200 || code >= 300 {
		return nil, ListProfilesOutput{}, fmt.Errorf("AlertHub API error (HTTP %d): %s", code, string(respBytes))
	}

	rawProfiles, err := decodeResponse[ProfileSummary](respBytes)
	if err != nil {
		return nil, ListProfilesOutput{}, err
	}

	filtered := make([]ProfileSummary, 0, len(rawProfiles))
	for _, ev := range rawProfiles {
		if !defaultsFilter && strings.EqualFold(strings.TrimSpace(ev.UserID), "admin") {
			continue
		}

		filtered = append(filtered, ev)
		if len(filtered) >= limit {
			break
		}
	}

	return nil, ListProfilesOutput{
		Count:    len(filtered),
		Profiles: filtered,
	}, nil
}
