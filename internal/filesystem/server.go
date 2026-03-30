package filesystem

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/erayyal/serveray-mcp/internal/shared/buildinfo"
	sharedconfig "github.com/erayyal/serveray-mcp/internal/shared/config"
	"github.com/erayyal/serveray-mcp/internal/shared/fsguard"
	"github.com/erayyal/serveray-mcp/internal/shared/limits"
	"github.com/erayyal/serveray-mcp/internal/shared/logging"
	"github.com/erayyal/serveray-mcp/internal/shared/mcpserver"
)

const (
	Name    = "filesystem-mcp"
	Version = buildinfo.Version
)

type Config struct {
	Roots        []string
	AllowHidden  bool
	MaxEntries   int
	MaxFileBytes int64
	MaxLines     int
}

func LoadConfig() (Config, error) {
	allowHidden, err := sharedconfig.Bool("FILESYSTEM_ALLOW_HIDDEN", false)
	if err != nil {
		return Config{}, err
	}
	maxEntries, err := sharedconfig.Int("FILESYSTEM_MAX_ENTRIES", 200, 1, 1000)
	if err != nil {
		return Config{}, err
	}
	maxFileBytes, err := sharedconfig.Int("FILESYSTEM_MAX_FILE_BYTES", 262144, 1024, 1048576)
	if err != nil {
		return Config{}, err
	}
	maxLines, err := sharedconfig.Int("FILESYSTEM_MAX_LINES", 500, 1, 5000)
	if err != nil {
		return Config{}, err
	}

	roots := sharedconfig.StringSlice("FILESYSTEM_ALLOWED_ROOTS")
	if len(roots) == 0 {
		return Config{}, fmt.Errorf("FILESYSTEM_ALLOWED_ROOTS is required")
	}

	return Config{
		Roots:        roots,
		AllowHidden:  allowHidden,
		MaxEntries:   maxEntries,
		MaxFileBytes: int64(maxFileBytes),
		MaxLines:     maxLines,
	}, nil
}

func New(ctx context.Context, logger *logging.Logger) (*server.MCPServer, error) {
	_ = ctx

	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	resolver, err := fsguard.New(cfg.Roots, cfg.AllowHidden)
	if err != nil {
		return nil, err
	}

	srv := mcpserver.New(Name, Version, "Filesystem MCP server. Access is restricted to configured roots and read-only tools.", logger)

	srv.AddTool(listRootsTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcpserver.JSONResult(resolver.ListRoots())
	})

	srv.AddTool(listDirectoryTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		root, err := request.RequireString("root")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		path := request.GetString("path", ".")
		limit := limits.Clamp(request.GetInt("limit", 0), cfg.MaxEntries, 1, cfg.MaxEntries)
		items, err := resolver.ListDir(root, path, limit)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(items)
	})

	srv.AddTool(readTextFileTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		root, err := request.RequireString("root")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		path, err := request.RequireString("path")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		startLine := limits.Clamp(request.GetInt("start_line", 1), 1, 1, 100000)
		maxLines := limits.Clamp(request.GetInt("max_lines", 0), cfg.MaxLines, 1, cfg.MaxLines)
		text, err := resolver.ReadTextFile(root, path, startLine, maxLines, cfg.MaxFileBytes)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	srv.AddTool(statPathTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		root, err := request.RequireString("root")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		path := request.GetString("path", ".")
		resolved, err := resolver.Resolve(root, path)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(map[string]any{
			"root":        root,
			"path":        path,
			"resolved":    resolved,
			"is_dir":      info.IsDir(),
			"size":        info.Size(),
			"mode":        info.Mode().String(),
			"modified_at": info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
		})
	})

	if err := mcpserver.AddJSONResource(srv, "server://filesystem/capabilities", "filesystem-capabilities", "Effective filesystem server safety configuration.", map[string]any{
		"server":         Name,
		"version":        Version,
		"default_mode":   "read-only safe mode",
		"allow_hidden":   cfg.AllowHidden,
		"allowed_roots":  resolver.ListRoots(),
		"max_entries":    cfg.MaxEntries,
		"max_file_bytes": cfg.MaxFileBytes,
		"max_lines":      cfg.MaxLines,
		"notes": []string{
			"Absolute paths from the client are rejected.",
			"Path traversal and symlink escapes are blocked.",
			"Roots are enforced server-side; client-provided roots are not trusted.",
		},
	}); err != nil {
		return nil, err
	}

	mcpserver.AddStaticPrompt(srv, "safe_usage", "Filesystem safe usage guidance.", "Use list_allowed_roots first, then provide a root alias plus a relative path. Hidden paths are blocked unless explicitly enabled in server configuration.")
	return srv, nil
}

func listRootsTool() mcp.Tool {
	return mcp.NewTool("list_allowed_roots",
		mcp.WithDescription("List configured filesystem roots and their aliases."),
	)
}

func listDirectoryTool() mcp.Tool {
	return mcp.NewTool("list_directory",
		mcp.WithDescription("List directory entries under a configured root."),
		mcp.WithString("root", mcp.Required(), mcp.Description("Configured root alias.")),
		mcp.WithString("path", mcp.Description("Relative path under the root. Defaults to '.'. Absolute paths are rejected.")),
		mcp.WithNumber("limit", mcp.Description("Optional lower cap on entries returned.")),
	)
}

func readTextFileTool() mcp.Tool {
	return mcp.NewTool("read_text_file",
		mcp.WithDescription("Read a bounded slice of a text file under a configured root."),
		mcp.WithString("root", mcp.Required(), mcp.Description("Configured root alias.")),
		mcp.WithString("path", mcp.Required(), mcp.Description("Relative path to the file.")),
		mcp.WithNumber("start_line", mcp.Description("1-based start line. Defaults to 1.")),
		mcp.WithNumber("max_lines", mcp.Description("Optional lower cap on lines returned.")),
	)
}

func statPathTool() mcp.Tool {
	return mcp.NewTool("stat_path",
		mcp.WithDescription("Stat a file or directory under a configured root."),
		mcp.WithString("root", mcp.Required(), mcp.Description("Configured root alias.")),
		mcp.WithString("path", mcp.Description("Relative path under the root. Defaults to '.'. Absolute paths are rejected.")),
	)
}
