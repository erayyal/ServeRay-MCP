package jira

import (
	"context"
	"encoding/base64"
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
	"github.com/erayyal/serveray-mcp/internal/shared/validation"
)

const (
	Name    = "jira-mcp"
	Version = buildinfo.Version
)

var issueKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)

type Config struct {
	BaseURL         string
	Email           string
	APIToken        string
	AllowedProjects []string
	MaxItems        int
	MinInterval     time.Duration
	Timeout         time.Duration
	AllowHTTP       bool
	AllowPrivate    bool
}

type Service struct {
	cfg     Config
	client  *httpx.Client
	allowed map[string]struct{}
}

type jiraSearchResponse struct {
	Issues []jiraIssue `json:"issues"`
}

type jiraIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Updated string `json:"updated"`
		Status  struct {
			Name string `json:"name"`
		} `json:"status"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Assignee *struct {
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
		Project struct {
			Key string `json:"key"`
		} `json:"project"`
	} `json:"fields"`
}

func LoadConfig() (Config, error) {
	baseURL, err := sharedconfig.RequiredString("JIRA_BASE_URL")
	if err != nil {
		return Config{}, err
	}
	if err := validation.HTTPURL(baseURL); err != nil {
		return Config{}, fmt.Errorf("JIRA_BASE_URL: %w", err)
	}
	email, err := sharedconfig.RequiredString("JIRA_EMAIL")
	if err != nil {
		return Config{}, err
	}
	apiToken, err := sharedconfig.RequiredString("JIRA_API_TOKEN")
	if err != nil {
		return Config{}, err
	}
	allowedProjects := sharedconfig.StringSlice("JIRA_ALLOWED_PROJECTS")
	if len(allowedProjects) == 0 {
		return Config{}, fmt.Errorf("JIRA_ALLOWED_PROJECTS is required")
	}
	maxItems, err := sharedconfig.Int("JIRA_MAX_ITEMS", 50, 1, 100)
	if err != nil {
		return Config{}, err
	}
	timeout, err := sharedconfig.Duration("JIRA_HTTP_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	minInterval, err := sharedconfig.Duration("JIRA_MIN_INTERVAL", 250*time.Millisecond)
	if err != nil {
		return Config{}, err
	}
	allowHTTP, err := sharedconfig.Bool("JIRA_ALLOW_INSECURE_HTTP", false)
	if err != nil {
		return Config{}, err
	}
	allowPrivate, err := sharedconfig.Bool("JIRA_ALLOW_PRIVATE_HOSTS", false)
	if err != nil {
		return Config{}, err
	}

	for _, project := range allowedProjects {
		project = strings.ToUpper(strings.TrimSpace(project))
		if project == "" {
			return Config{}, fmt.Errorf("empty JIRA_ALLOWED_PROJECTS entry")
		}
	}

	return Config{
		BaseURL:         baseURL,
		Email:           email,
		APIToken:        apiToken,
		AllowedProjects: allowedProjects,
		MaxItems:        maxItems,
		MinInterval:     minInterval,
		Timeout:         timeout,
		AllowHTTP:       allowHTTP,
		AllowPrivate:    allowPrivate,
	}, nil
}

func New(ctx context.Context, logger *logging.Logger) (*server.MCPServer, error) {
	_ = ctx

	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(cfg.Email + ":" + cfg.APIToken))
	client, err := httpx.New(httpx.Config{
		BaseURL:           cfg.BaseURL,
		Timeout:           cfg.Timeout,
		UserAgent:         Name + "/" + Version,
		AuthHeader:        "Basic " + auth,
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
			out := make(map[string]struct{}, len(cfg.AllowedProjects))
			for _, project := range cfg.AllowedProjects {
				out[strings.ToUpper(project)] = struct{}{}
			}
			return out
		}(),
	}

	srv := mcpserver.New(Name, Version, "Jira MCP server. Only configured projects are accessible. Read-only tools only.", logger)

	srv.AddTool(listAllowedProjectsTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcpserver.JSONResult(cfg.AllowedProjects)
	})

	srv.AddTool(searchIssuesTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := request.RequireString("project")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		status := request.GetString("status", "")
		assignee := request.GetString("assignee", "")
		text := request.GetString("text", "")
		limit := limits.Clamp(request.GetInt("limit", 0), cfg.MaxItems, 1, cfg.MaxItems)
		items, err := service.SearchIssues(ctx, project, status, assignee, text, limit)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(items)
	})

	srv.AddTool(getIssueTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := request.RequireString("issue_key")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		item, err := service.GetIssue(ctx, key)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(item)
	})

	if err := mcpserver.AddJSONResource(srv, "server://jira/capabilities", "jira-capabilities", "Effective Jira server safety configuration.", map[string]any{
		"server":           Name,
		"version":          Version,
		"default_mode":     "read-only safe mode",
		"allowed_projects": cfg.AllowedProjects,
		"max_items":        cfg.MaxItems,
		"request_timeout":  cfg.Timeout.String(),
		"minimum_interval": cfg.MinInterval.String(),
		"write_tools":      false,
		"required_scopes":  []string{"Browse projects and issues"},
		"security_principles": []string{
			"Only configured project keys are accessible.",
			"Issue search is structured and server-built instead of accepting arbitrary JQL.",
			"Remote requests are rate-limited and retried only for safe GET operations.",
		},
	}); err != nil {
		return nil, err
	}

	mcpserver.AddStaticPrompt(srv, "safe_usage", "Jira safe usage guidance.", "Use list_allowed_projects first. search_issues only queries configured projects and builds JQL server-side; arbitrary JQL is intentionally not exposed.")
	return srv, nil
}

func (s *Service) SearchIssues(ctx context.Context, project, status, assignee, text string, limit int) ([]map[string]any, error) {
	project = strings.ToUpper(strings.TrimSpace(project))
	if err := s.validateProject(project); err != nil {
		return nil, err
	}

	clauses := []string{fmt.Sprintf(`project = "%s"`, project)}
	if status != "" {
		clauses = append(clauses, fmt.Sprintf(`status = "%s"`, escapeJQL(status)))
	}
	if assignee != "" {
		clauses = append(clauses, fmt.Sprintf(`assignee = "%s"`, escapeJQL(assignee)))
	}
	if text != "" {
		clauses = append(clauses, fmt.Sprintf(`text ~ "\"%s\""`, escapeJQL(text)))
	}

	query := url.Values{
		"jql":        []string{strings.Join(clauses, " AND ")},
		"maxResults": []string{strconv.Itoa(limit)},
		"fields":     []string{"summary,status,issuetype,assignee,updated,project"},
	}

	var response jiraSearchResponse
	if err := s.client.DoJSON(ctx, "/rest/api/3/search", query, map[string]string{
		"Accept": "application/json",
	}, &response); err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(response.Issues))
	for _, issue := range response.Issues {
		items = append(items, summarizeIssue(issue))
	}
	return items, nil
}

func (s *Service) GetIssue(ctx context.Context, key string) (map[string]any, error) {
	key = strings.ToUpper(strings.TrimSpace(key))
	if !issueKeyPattern.MatchString(key) {
		return nil, fmt.Errorf("issue_key must look like ABC-123")
	}
	project := strings.SplitN(key, "-", 2)[0]
	if err := s.validateProject(project); err != nil {
		return nil, err
	}

	query := url.Values{"fields": []string{"summary,status,issuetype,assignee,updated,project"}}
	var issue jiraIssue
	if err := s.client.DoJSON(ctx, "/rest/api/3/issue/"+key, query, map[string]string{
		"Accept": "application/json",
	}, &issue); err != nil {
		return nil, err
	}
	return summarizeIssue(issue), nil
}

func (s *Service) validateProject(project string) error {
	if _, ok := s.allowed[project]; !ok {
		return fmt.Errorf("project %q is not in JIRA_ALLOWED_PROJECTS", project)
	}
	return nil
}

func summarizeIssue(issue jiraIssue) map[string]any {
	assignee := ""
	if issue.Fields.Assignee != nil {
		assignee = issue.Fields.Assignee.DisplayName
	}
	return map[string]any{
		"key":        issue.Key,
		"project":    issue.Fields.Project.Key,
		"summary":    issue.Fields.Summary,
		"status":     issue.Fields.Status.Name,
		"issue_type": issue.Fields.IssueType.Name,
		"assignee":   assignee,
		"updated_at": issue.Fields.Updated,
	}
}

func escapeJQL(value string) string {
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}

func listAllowedProjectsTool() mcp.Tool {
	return mcp.NewTool("list_allowed_projects",
		mcp.WithDescription("List the Jira project keys this server is allowed to access."),
	)
}

func searchIssuesTool() mcp.Tool {
	return mcp.NewTool("search_issues",
		mcp.WithDescription("Search issues within an allowlisted Jira project using structured filters."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Allowlisted Jira project key.")),
		mcp.WithString("status", mcp.Description("Optional status filter.")),
		mcp.WithString("assignee", mcp.Description("Optional assignee filter.")),
		mcp.WithString("text", mcp.Description("Optional text search applied to Jira's text field.")),
		mcp.WithNumber("limit", mcp.Description("Optional lower cap on issues returned.")),
	)
}

func getIssueTool() mcp.Tool {
	return mcp.NewTool("get_issue",
		mcp.WithDescription("Get a single Jira issue from an allowlisted project."),
		mcp.WithString("issue_key", mcp.Required(), mcp.Description("Issue key like ABC-123.")),
	)
}
