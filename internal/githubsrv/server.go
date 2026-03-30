package githubsrv

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
	"github.com/erayyal/serveray-mcp/internal/shared/validation"
)

const (
	Name    = "github-mcp"
	Version = buildinfo.Version
)

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

type Config struct {
	Token        string
	BaseURL      string
	AllowedRepos []string
	MaxItems     int
	MinInterval  time.Duration
	Timeout      time.Duration
	AllowHTTP    bool
	AllowPrivate bool
}

type Service struct {
	cfg     Config
	client  *httpx.Client
	allowed map[string]struct{}
}

type githubIssue struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	HTMLURL   string `json:"html_url"`
	UpdatedAt string `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	PullRequest *struct{} `json:"pull_request"`
}

type githubPullRequest struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	HTMLURL   string `json:"html_url"`
	UpdatedAt string `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}

func LoadConfig() (Config, error) {
	token, err := sharedconfig.RequiredString("GITHUB_TOKEN")
	if err != nil {
		return Config{}, err
	}
	baseURL := sharedconfig.String("GITHUB_BASE_URL", "https://api.github.com/")
	allowedRepos := sharedconfig.StringSlice("GITHUB_ALLOWED_REPOS")
	if len(allowedRepos) == 0 {
		return Config{}, fmt.Errorf("GITHUB_ALLOWED_REPOS is required")
	}
	maxItems, err := sharedconfig.Int("GITHUB_MAX_ITEMS", 50, 1, 100)
	if err != nil {
		return Config{}, err
	}
	timeout, err := sharedconfig.Duration("GITHUB_HTTP_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	minInterval, err := sharedconfig.Duration("GITHUB_MIN_INTERVAL", 250*time.Millisecond)
	if err != nil {
		return Config{}, err
	}
	allowHTTP, err := sharedconfig.Bool("GITHUB_ALLOW_INSECURE_HTTP", false)
	if err != nil {
		return Config{}, err
	}
	allowPrivate, err := sharedconfig.Bool("GITHUB_ALLOW_PRIVATE_HOSTS", false)
	if err != nil {
		return Config{}, err
	}

	for _, repo := range allowedRepos {
		if !repoPattern.MatchString(repo) {
			return Config{}, fmt.Errorf("invalid GITHUB_ALLOWED_REPOS entry %q", repo)
		}
	}

	return Config{
		Token:        token,
		BaseURL:      baseURL,
		AllowedRepos: allowedRepos,
		MaxItems:     maxItems,
		MinInterval:  minInterval,
		Timeout:      timeout,
		AllowHTTP:    allowHTTP,
		AllowPrivate: allowPrivate,
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
			out := make(map[string]struct{}, len(cfg.AllowedRepos))
			for _, repo := range cfg.AllowedRepos {
				out[repo] = struct{}{}
			}
			return out
		}(),
	}

	srv := mcpserver.New(Name, Version, "GitHub MCP server. Only configured repositories are accessible. Read-only tools only.", logger)

	srv.AddTool(listAllowedRepositoriesTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcpserver.JSONResult(cfg.AllowedRepos)
	})

	srv.AddTool(listIssuesTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repo, err := request.RequireString("repo")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		state := request.GetString("state", "open")
		if err := validation.OneOf("state", state, "open", "closed", "all"); err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		limit := limits.Clamp(request.GetInt("limit", 0), cfg.MaxItems, 1, cfg.MaxItems)
		items, err := service.ListIssues(ctx, repo, state, limit)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(items)
	})

	srv.AddTool(getIssueTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repo, err := request.RequireString("repo")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		number, err := request.RequireInt("number")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		item, err := service.GetIssue(ctx, repo, number)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(item)
	})

	srv.AddTool(listPullRequestsTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repo, err := request.RequireString("repo")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		state := request.GetString("state", "open")
		if err := validation.OneOf("state", state, "open", "closed", "all"); err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		limit := limits.Clamp(request.GetInt("limit", 0), cfg.MaxItems, 1, cfg.MaxItems)
		items, err := service.ListPullRequests(ctx, repo, state, limit)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(items)
	})

	if err := mcpserver.AddJSONResource(srv, "server://github/capabilities", "github-capabilities", "Effective GitHub server safety configuration.", map[string]any{
		"server":           Name,
		"version":          Version,
		"default_mode":     "read-only safe mode",
		"allowed_repos":    cfg.AllowedRepos,
		"max_items":        cfg.MaxItems,
		"request_timeout":  cfg.Timeout.String(),
		"minimum_interval": cfg.MinInterval.String(),
		"write_tools":      false,
		"required_scopes":  []string{"issues:read", "pull_requests:read", "metadata:read"},
		"security_principles": []string{
			"Only configured repositories are accessible.",
			"No arbitrary endpoint tool is exposed.",
			"Remote requests are rate-limited and retried only for safe GET operations.",
		},
	}); err != nil {
		return nil, err
	}

	mcpserver.AddStaticPrompt(srv, "safe_usage", "GitHub safe usage guidance.", "Use list_allowed_repositories first, then pass one of those repository identifiers into list_issues, get_issue, or list_pull_requests. Write operations are intentionally absent.")
	return srv, nil
}

func (s *Service) ListIssues(ctx context.Context, repo, state string, limit int) ([]map[string]any, error) {
	if err := s.validateRepo(repo); err != nil {
		return nil, err
	}

	query := url.Values{
		"state":    []string{state},
		"per_page": []string{strconv.Itoa(limit)},
	}
	var response []githubIssue
	if err := s.client.DoJSON(ctx, "/repos/"+repo+"/issues", query, map[string]string{
		"Accept": "application/vnd.github+json",
	}, &response); err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(response))
	for _, issue := range response {
		if issue.PullRequest != nil {
			continue
		}
		labels := make([]string, 0, len(issue.Labels))
		for _, label := range issue.Labels {
			labels = append(labels, label.Name)
		}
		items = append(items, map[string]any{
			"number":     issue.Number,
			"title":      issue.Title,
			"state":      issue.State,
			"url":        issue.HTMLURL,
			"updated_at": issue.UpdatedAt,
			"author":     issue.User.Login,
			"labels":     labels,
		})
	}
	return items, nil
}

func (s *Service) GetIssue(ctx context.Context, repo string, number int) (map[string]any, error) {
	if err := s.validateRepo(repo); err != nil {
		return nil, err
	}

	var issue githubIssue
	if err := s.client.DoJSON(ctx, fmt.Sprintf("/repos/%s/issues/%d", repo, number), nil, map[string]string{
		"Accept": "application/vnd.github+json",
	}, &issue); err != nil {
		return nil, err
	}

	labels := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		labels = append(labels, label.Name)
	}

	return map[string]any{
		"number":     issue.Number,
		"title":      issue.Title,
		"state":      issue.State,
		"url":        issue.HTMLURL,
		"updated_at": issue.UpdatedAt,
		"author":     issue.User.Login,
		"labels":     labels,
	}, nil
}

func (s *Service) ListPullRequests(ctx context.Context, repo, state string, limit int) ([]map[string]any, error) {
	if err := s.validateRepo(repo); err != nil {
		return nil, err
	}

	query := url.Values{
		"state":    []string{state},
		"per_page": []string{strconv.Itoa(limit)},
	}
	var response []githubPullRequest
	if err := s.client.DoJSON(ctx, "/repos/"+repo+"/pulls", query, map[string]string{
		"Accept": "application/vnd.github+json",
	}, &response); err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(response))
	for _, pr := range response {
		items = append(items, map[string]any{
			"number":     pr.Number,
			"title":      pr.Title,
			"state":      pr.State,
			"url":        pr.HTMLURL,
			"updated_at": pr.UpdatedAt,
			"author":     pr.User.Login,
		})
	}
	return items, nil
}

func (s *Service) validateRepo(repo string) error {
	repo = strings.TrimSpace(repo)
	if !repoPattern.MatchString(repo) {
		return fmt.Errorf("repo must be owner/name")
	}
	if _, ok := s.allowed[repo]; !ok {
		return fmt.Errorf("repo %q is not in GITHUB_ALLOWED_REPOS", repo)
	}
	return nil
}

func listAllowedRepositoriesTool() mcp.Tool {
	return mcp.NewTool("list_allowed_repositories",
		mcp.WithDescription("List the repositories this GitHub MCP server is allowed to access."),
	)
}

func listIssuesTool() mcp.Tool {
	return mcp.NewTool("list_issues",
		mcp.WithDescription("List issues for an allowlisted repository."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository in owner/name form. Must be allowlisted.")),
		mcp.WithString("state", mcp.Description("Issue state: open, closed, or all. Defaults to open.")),
		mcp.WithNumber("limit", mcp.Description("Optional lower cap on issues returned.")),
	)
}

func getIssueTool() mcp.Tool {
	return mcp.NewTool("get_issue",
		mcp.WithDescription("Get a single issue from an allowlisted repository."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository in owner/name form. Must be allowlisted.")),
		mcp.WithNumber("number", mcp.Required(), mcp.Description("Issue number.")),
	)
}

func listPullRequestsTool() mcp.Tool {
	return mcp.NewTool("list_pull_requests",
		mcp.WithDescription("List pull requests for an allowlisted repository."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository in owner/name form. Must be allowlisted.")),
		mcp.WithString("state", mcp.Description("Pull request state: open, closed, or all. Defaults to open.")),
		mcp.WithNumber("limit", mcp.Description("Optional lower cap on pull requests returned.")),
	)
}
