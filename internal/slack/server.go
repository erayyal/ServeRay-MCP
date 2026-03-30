package slack

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/erayyal/serveray-mcp/internal/shared/buildinfo"
	sharedconfig "github.com/erayyal/serveray-mcp/internal/shared/config"
	"github.com/erayyal/serveray-mcp/internal/shared/httpx"
	"github.com/erayyal/serveray-mcp/internal/shared/limits"
	"github.com/erayyal/serveray-mcp/internal/shared/logging"
	"github.com/erayyal/serveray-mcp/internal/shared/mcpserver"
)

const (
	Name    = "slack-mcp"
	Version = buildinfo.Version
)

var channelPattern = regexp.MustCompile(`^[A-Z0-9]+$`)

type Config struct {
	Token             string
	BaseURL           string
	AllowedChannelIDs []string
	MaxItems          int
	MinInterval       time.Duration
	Timeout           time.Duration
	AllowHTTP         bool
	AllowPrivate      bool
}

type Service struct {
	cfg     Config
	client  *httpx.Client
	allowed map[string]struct{}
}

type slackAPIError struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

type slackHistoryResponse struct {
	slackAPIError
	Messages []slackMessage `json:"messages"`
}

type slackMessage struct {
	Type     string `json:"type"`
	User     string `json:"user"`
	Text     string `json:"text"`
	TS       string `json:"ts"`
	ThreadTS string `json:"thread_ts"`
}

func LoadConfig() (Config, error) {
	token, err := sharedconfig.RequiredString("SLACK_BOT_TOKEN")
	if err != nil {
		return Config{}, err
	}
	baseURL := sharedconfig.String("SLACK_BASE_URL", "https://slack.com/api/")
	allowedChannels := sharedconfig.StringSlice("SLACK_ALLOWED_CHANNEL_IDS")
	if len(allowedChannels) == 0 {
		return Config{}, fmt.Errorf("SLACK_ALLOWED_CHANNEL_IDS is required")
	}
	maxItems, err := sharedconfig.Int("SLACK_MAX_ITEMS", 50, 1, 100)
	if err != nil {
		return Config{}, err
	}
	timeout, err := sharedconfig.Duration("SLACK_HTTP_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	minInterval, err := sharedconfig.Duration("SLACK_MIN_INTERVAL", 300*time.Millisecond)
	if err != nil {
		return Config{}, err
	}
	allowHTTP, err := sharedconfig.Bool("SLACK_ALLOW_INSECURE_HTTP", false)
	if err != nil {
		return Config{}, err
	}
	allowPrivate, err := sharedconfig.Bool("SLACK_ALLOW_PRIVATE_HOSTS", false)
	if err != nil {
		return Config{}, err
	}

	for _, channelID := range allowedChannels {
		if !channelPattern.MatchString(channelID) {
			return Config{}, fmt.Errorf("invalid channel id %q in SLACK_ALLOWED_CHANNEL_IDS", channelID)
		}
	}

	return Config{
		Token:             token,
		BaseURL:           baseURL,
		AllowedChannelIDs: allowedChannels,
		MaxItems:          maxItems,
		MinInterval:       minInterval,
		Timeout:           timeout,
		AllowHTTP:         allowHTTP,
		AllowPrivate:      allowPrivate,
	}, nil
}

func New(ctx context.Context, logger *logging.Logger) (*server.MCPServer, error) {
	_ = ctx

	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	client, err := httpx.New(httpx.Config{
		BaseURL:           cfg.BaseURL,
		Timeout:           cfg.Timeout,
		UserAgent:         Name + "/" + Version,
		AuthHeader:        "Bearer " + cfg.Token,
		MinInterval:       cfg.MinInterval,
		MaxRetries:        2,
		ResponseSize:      1 << 20,
		AllowHTTP:         cfg.AllowHTTP,
		AllowPrivateHosts: cfg.AllowPrivate,
	})
	if err != nil {
		return nil, err
	}

	service := &Service{
		cfg:    cfg,
		client: client,
		allowed: func() map[string]struct{} {
			out := make(map[string]struct{}, len(cfg.AllowedChannelIDs))
			for _, channelID := range cfg.AllowedChannelIDs {
				out[channelID] = struct{}{}
			}
			return out
		}(),
	}

	srv := mcpserver.New(Name, Version, "Slack MCP server. Only configured channels are accessible. Read-only tools only.", logger)

	srv.AddTool(listAllowedChannelsTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		items, err := service.ListAllowedChannels(ctx)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(items)
	})

	srv.AddTool(channelHistoryTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		channelID, err := request.RequireString("channel_id")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		limit := limits.Clamp(request.GetInt("limit", 0), cfg.MaxItems, 1, cfg.MaxItems)
		items, err := service.ChannelHistory(ctx, channelID, limit)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(items)
	})

	srv.AddTool(getThreadTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		channelID, err := request.RequireString("channel_id")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		threadTS, err := request.RequireString("thread_ts")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		limit := limits.Clamp(request.GetInt("limit", 0), cfg.MaxItems, 1, cfg.MaxItems)
		items, err := service.GetThread(ctx, channelID, threadTS, limit)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(items)
	})

	if err := mcpserver.AddJSONResource(srv, "server://slack/capabilities", "slack-capabilities", "Effective Slack server safety configuration.", map[string]any{
		"server":           Name,
		"version":          Version,
		"default_mode":     "read-only safe mode",
		"allowed_channels": cfg.AllowedChannelIDs,
		"max_items":        cfg.MaxItems,
		"request_timeout":  cfg.Timeout.String(),
		"minimum_interval": cfg.MinInterval.String(),
		"write_tools":      false,
		"required_scopes":  []string{"channels:read", "groups:read", "channels:history", "groups:history"},
		"security_principles": []string{
			"Only configured channel IDs are accessible.",
			"No posting or mutation tools are exposed.",
			"Remote requests are rate-limited and retried only for safe GET operations.",
		},
	}); err != nil {
		return nil, err
	}

	mcpserver.AddStaticPrompt(srv, "safe_usage", "Slack safe usage guidance.", "Use list_allowed_channels first. Provide one of those channel IDs to channel_history or get_thread. Posting messages is intentionally unsupported.")
	return srv, nil
}

func (s *Service) ListAllowedChannels(ctx context.Context) ([]map[string]any, error) {
	_ = ctx

	items := make([]map[string]any, 0, len(s.cfg.AllowedChannelIDs))
	for _, channelID := range s.cfg.AllowedChannelIDs {
		items = append(items, map[string]any{
			"id":     channelID,
			"source": "configured_allowlist",
		})
	}
	return items, nil
}

func (s *Service) ChannelHistory(ctx context.Context, channelID string, limit int) ([]map[string]any, error) {
	if err := s.validateChannel(channelID); err != nil {
		return nil, err
	}
	query := url.Values{
		"channel": []string{channelID},
		"limit":   []string{strconv.Itoa(limit)},
	}
	var response slackHistoryResponse
	if err := s.client.DoJSON(ctx, "/conversations.history", query, nil, &response); err != nil {
		return nil, err
	}
	if !response.OK {
		return nil, fmt.Errorf("slack API error: %s", response.Error)
	}
	return summarizeMessages(response.Messages), nil
}

func (s *Service) GetThread(ctx context.Context, channelID, threadTS string, limit int) ([]map[string]any, error) {
	if err := s.validateChannel(channelID); err != nil {
		return nil, err
	}
	query := url.Values{
		"channel":   []string{channelID},
		"ts":        []string{threadTS},
		"limit":     []string{strconv.Itoa(limit)},
		"inclusive": []string{"true"},
	}
	var response slackHistoryResponse
	if err := s.client.DoJSON(ctx, "/conversations.replies", query, nil, &response); err != nil {
		return nil, err
	}
	if !response.OK {
		return nil, fmt.Errorf("slack API error: %s", response.Error)
	}
	return summarizeMessages(response.Messages), nil
}

func (s *Service) validateChannel(channelID string) error {
	channelID = strings.TrimSpace(channelID)
	if _, ok := s.allowed[channelID]; !ok {
		return fmt.Errorf("channel %q is not in SLACK_ALLOWED_CHANNEL_IDS", channelID)
	}
	return nil
}

func summarizeMessages(messages []slackMessage) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		text, _ := limits.TruncateString(message.Text, 4000)
		out = append(out, map[string]any{
			"type":      message.Type,
			"user":      message.User,
			"text":      text,
			"ts":        message.TS,
			"thread_ts": message.ThreadTS,
		})
	}
	return out
}

func listAllowedChannelsTool() mcp.Tool {
	return mcp.NewTool("list_allowed_channels",
		mcp.WithDescription("List the Slack channels this server is allowed to access."),
	)
}

func channelHistoryTool() mcp.Tool {
	return mcp.NewTool("channel_history",
		mcp.WithDescription("Read recent messages from an allowlisted Slack channel."),
		mcp.WithString("channel_id", mcp.Required(), mcp.Description("Allowlisted Slack channel ID.")),
		mcp.WithNumber("limit", mcp.Description("Optional lower cap on messages returned.")),
	)
}

func getThreadTool() mcp.Tool {
	return mcp.NewTool("get_thread",
		mcp.WithDescription("Read messages from a Slack thread in an allowlisted channel."),
		mcp.WithString("channel_id", mcp.Required(), mcp.Description("Allowlisted Slack channel ID.")),
		mcp.WithString("thread_ts", mcp.Required(), mcp.Description("Slack thread timestamp.")),
		mcp.WithNumber("limit", mcp.Description("Optional lower cap on replies returned.")),
	)
}
