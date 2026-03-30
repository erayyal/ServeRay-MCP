package playwright

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/erayyal/serveray-mcp/internal/shared/buildinfo"
	sharedconfig "github.com/erayyal/serveray-mcp/internal/shared/config"
	"github.com/erayyal/serveray-mcp/internal/shared/limits"
	"github.com/erayyal/serveray-mcp/internal/shared/logging"
	"github.com/erayyal/serveray-mcp/internal/shared/mcpserver"
	"github.com/erayyal/serveray-mcp/internal/shared/validation"
)

const (
	Name    = "playwright-mcp"
	Version = buildinfo.Version
)

type Config struct {
	AllowedOrigins    []string
	BrowserPath       string
	Headless          bool
	NavigationTimeout time.Duration
	MaxTextBytes      int
	MaxLinks          int
	EnableScreenshots bool
}

type Service struct {
	cfg            Config
	allowedOrigins map[string]struct{}
}

func LoadConfig() (Config, error) {
	allowedOrigins := sharedconfig.StringSlice("PLAYWRIGHT_ALLOWED_ORIGINS")
	if len(allowedOrigins) == 0 {
		return Config{}, fmt.Errorf("PLAYWRIGHT_ALLOWED_ORIGINS is required")
	}
	for _, origin := range allowedOrigins {
		if err := validation.HTTPURL(origin); err != nil {
			return Config{}, fmt.Errorf("invalid allowlisted origin %q: %w", origin, err)
		}
	}

	headless, err := sharedconfig.Bool("PLAYWRIGHT_HEADLESS", true)
	if err != nil {
		return Config{}, err
	}
	navigationTimeout, err := sharedconfig.Duration("PLAYWRIGHT_NAV_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	maxTextBytes, err := sharedconfig.Int("PLAYWRIGHT_MAX_TEXT_BYTES", 32768, 1024, 262144)
	if err != nil {
		return Config{}, err
	}
	maxLinks, err := sharedconfig.Int("PLAYWRIGHT_MAX_LINKS", 100, 1, 500)
	if err != nil {
		return Config{}, err
	}
	enableScreenshots, err := sharedconfig.Bool("PLAYWRIGHT_ENABLE_SCREENSHOTS", false)
	if err != nil {
		return Config{}, err
	}

	return Config{
		AllowedOrigins:    allowedOrigins,
		BrowserPath:       sharedconfig.String("PLAYWRIGHT_BROWSER_PATH", ""),
		Headless:          headless,
		NavigationTimeout: navigationTimeout,
		MaxTextBytes:      maxTextBytes,
		MaxLinks:          maxLinks,
		EnableScreenshots: enableScreenshots,
	}, nil
}

func New(ctx context.Context, logger *logging.Logger) (*server.MCPServer, error) {
	_ = ctx

	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	service := &Service{
		cfg:            cfg,
		allowedOrigins: make(map[string]struct{}, len(cfg.AllowedOrigins)),
	}
	for _, origin := range cfg.AllowedOrigins {
		service.allowedOrigins[normalizeOrigin(origin)] = struct{}{}
	}

	srv := mcpserver.New(Name, Version, "Go-first browser automation server with Playwright-style guardrails. Navigation is restricted to configured origins.", logger)

	srv.AddTool(fetchPageTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rawURL, err := request.RequireString("url")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		page, err := service.FetchPage(ctx, rawURL)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(page)
	})

	srv.AddTool(extractLinksTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rawURL, err := request.RequireString("url")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		limit := limits.Clamp(request.GetInt("limit", 0), cfg.MaxLinks, 1, cfg.MaxLinks)
		items, err := service.ExtractLinks(ctx, rawURL, limit)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(items)
	})

	if cfg.EnableScreenshots {
		srv.AddTool(captureScreenshotTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			rawURL, err := request.RequireString("url")
			if err != nil {
				return mcpserver.ErrorResult(err.Error()), nil
			}
			imageData, finalURL, err := service.CaptureScreenshot(ctx, rawURL)
			if err != nil {
				return mcpserver.ErrorResult(err.Error()), nil
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Screenshot captured for " + finalURL),
					mcp.NewImageContent(base64.StdEncoding.EncodeToString(imageData), "image/png"),
				},
			}, nil
		})
	}

	if err := mcpserver.AddJSONResource(srv, "server://playwright/capabilities", "playwright-capabilities", "Effective browser server safety configuration.", map[string]any{
		"server":             Name,
		"version":            Version,
		"implementation":     "Go + chromedp (not the Node Playwright runtime)",
		"default_mode":       "allowlisted navigation only",
		"allowed_origins":    cfg.AllowedOrigins,
		"navigation_timeout": cfg.NavigationTimeout.String(),
		"max_text_bytes":     cfg.MaxTextBytes,
		"max_links":          cfg.MaxLinks,
		"enable_screenshots": cfg.EnableScreenshots,
		"security_principles": []string{
			"Only configured origins are reachable.",
			"No arbitrary script execution tool is exposed.",
			"Downloads and uploads are intentionally unsupported in this release.",
		},
	}); err != nil {
		return nil, err
	}

	mcpserver.AddStaticPrompt(srv, "safe_usage", "Browser safe usage guidance.", "This server only visits allowlisted origins. Use fetch_page for bounded text extraction and extract_links for safe link enumeration. Screenshot capture is hidden unless explicitly enabled.")
	return srv, nil
}

func (s *Service) FetchPage(ctx context.Context, rawURL string) (map[string]any, error) {
	targetURL, err := s.validateURL(rawURL)
	if err != nil {
		return nil, err
	}

	var title string
	var bodyText string
	var finalURL string
	if err := s.run(ctx, func(taskCtx context.Context) error {
		return chromedp.Run(taskCtx,
			chromedp.Navigate(targetURL.String()),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.Location(&finalURL),
			chromedp.Title(&title),
			chromedp.Evaluate(`document.body ? document.body.innerText : ""`, &bodyText),
		)
	}); err != nil {
		return nil, err
	}

	if _, err := s.validateURL(finalURL); err != nil {
		return nil, fmt.Errorf("navigation redirected outside the allowlist: %w", err)
	}

	bodyText, _ = limits.TruncateString(bodyText, s.cfg.MaxTextBytes)
	return map[string]any{
		"url":       finalURL,
		"title":     title,
		"body_text": bodyText,
	}, nil
}

func (s *Service) ExtractLinks(ctx context.Context, rawURL string, limit int) ([]map[string]any, error) {
	targetURL, err := s.validateURL(rawURL)
	if err != nil {
		return nil, err
	}

	var finalURL string
	var links []map[string]string
	if err := s.run(ctx, func(taskCtx context.Context) error {
		return chromedp.Run(taskCtx,
			chromedp.Navigate(targetURL.String()),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.Location(&finalURL),
			chromedp.Evaluate(fmt.Sprintf(`Array.from(document.links).slice(0, %d).map(link => ({text: (link.innerText || "").trim(), href: link.href}))`, limit), &links),
		)
	}); err != nil {
		return nil, err
	}

	if _, err := s.validateURL(finalURL); err != nil {
		return nil, fmt.Errorf("navigation redirected outside the allowlist: %w", err)
	}

	items := make([]map[string]any, 0, len(links))
	for _, link := range links {
		if link["href"] == "" {
			continue
		}
		text, _ := limits.TruncateString(link["text"], 256)
		items = append(items, map[string]any{
			"text": text,
			"href": link["href"],
		})
	}
	return items, nil
}

func (s *Service) CaptureScreenshot(ctx context.Context, rawURL string) ([]byte, string, error) {
	targetURL, err := s.validateURL(rawURL)
	if err != nil {
		return nil, "", err
	}

	var finalURL string
	var screenshot []byte
	if err := s.run(ctx, func(taskCtx context.Context) error {
		return chromedp.Run(taskCtx,
			chromedp.Navigate(targetURL.String()),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.Location(&finalURL),
			chromedp.CaptureScreenshot(&screenshot),
		)
	}); err != nil {
		return nil, "", err
	}

	if _, err := s.validateURL(finalURL); err != nil {
		return nil, "", fmt.Errorf("navigation redirected outside the allowlist: %w", err)
	}
	return screenshot, finalURL, nil
}

func (s *Service) validateURL(raw string) (*url.URL, error) {
	if err := validation.HTTPURL(raw); err != nil {
		return nil, err
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("URLs with embedded credentials are not allowed")
	}
	if _, ok := s.allowedOrigins[normalizeOrigin(parsed.String())]; !ok {
		return nil, fmt.Errorf("origin %q is not allowlisted", parsed.Scheme+"://"+parsed.Host)
	}
	return parsed, nil
}

func (s *Service) run(ctx context.Context, fn func(context.Context) error) error {
	taskCtx, cancel := context.WithTimeout(ctx, s.cfg.NavigationTimeout)
	defer cancel()

	allocatorOptions := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("headless", s.cfg.Headless),
		chromedp.Flag("mute-audio", true),
	)
	if s.cfg.BrowserPath != "" {
		allocatorOptions = append(allocatorOptions, chromedp.ExecPath(s.cfg.BrowserPath))
	}

	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(taskCtx, allocatorOptions...)
	defer cancelAllocator()

	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	defer cancelBrowser()

	return fn(browserCtx)
}

func normalizeOrigin(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}

func fetchPageTool() mcp.Tool {
	return mcp.NewTool("fetch_page",
		mcp.WithDescription("Navigate to an allowlisted URL and return bounded page text."),
		mcp.WithString("url", mcp.Required(), mcp.Description("Allowlisted http/https URL.")),
	)
}

func extractLinksTool() mcp.Tool {
	return mcp.NewTool("extract_links",
		mcp.WithDescription("Navigate to an allowlisted URL and extract a bounded list of links."),
		mcp.WithString("url", mcp.Required(), mcp.Description("Allowlisted http/https URL.")),
		mcp.WithNumber("limit", mcp.Description("Optional lower cap on links returned.")),
	)
}

func captureScreenshotTool() mcp.Tool {
	return mcp.NewTool("capture_screenshot",
		mcp.WithDescription("Capture a screenshot for an allowlisted URL. Hidden unless screenshots are explicitly enabled."),
		mcp.WithString("url", mcp.Required(), mcp.Description("Allowlisted http/https URL.")),
	)
}
