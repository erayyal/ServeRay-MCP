package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	sharedconfig "github.com/erayyal/serveray-mcp/internal/shared/config"
	"github.com/erayyal/serveray-mcp/internal/shared/logging"
	"github.com/erayyal/serveray-mcp/internal/shared/redaction"
)

var errorRedactor = redaction.New()

func New(name, version, instructions string, logger *logging.Logger) *server.MCPServer {
	options := []server.ServerOption{
		server.WithToolCapabilities(false),
		server.WithPromptCapabilities(false),
		server.WithResourceCapabilities(false, false),
		server.WithToolHandlerMiddleware(recoverToolMiddleware(logger)),
		server.WithToolHandlerMiddleware(auditToolMiddleware(logger)),
	}

	if instructions != "" {
		options = append(options, server.WithInstructions(instructions))
	}

	return server.NewMCPServer(name, version, options...)
}

func RunMain(component string, run func(context.Context, *logging.Logger) error) {
	ctx := context.Background()
	logger := logging.New(component, sharedconfig.String("LOG_LEVEL", "info"), redaction.New())

	defer func() {
		if recovered := recover(); recovered != nil {
			logger.Error(ctx, "panic recovered",
				slog.String("panic", fmt.Sprint(recovered)),
			)
			os.Exit(1)
		}
	}()

	if err := run(ctx, logger); err != nil {
		logger.Error(ctx, "fatal error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func ServeStdio(ctx context.Context, srv *server.MCPServer) error {
	stdio := server.NewStdioServer(srv)
	stdio.SetErrorLogger(log.New(os.Stderr, "", log.LstdFlags))
	return stdio.Listen(ctx, os.Stdin, os.Stdout)
}

func AddJSONResource(srv *server.MCPServer, uri, name, description string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal resource %s: %w", uri, err)
	}

	resource := mcp.NewResource(uri, name,
		mcp.WithResourceDescription(description),
		mcp.WithMIMEType("application/json"),
	)

	srv.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(payload),
			},
		}, nil
	})

	return nil
}

func AddStaticPrompt(srv *server.MCPServer, name, description, text string) {
	prompt := mcp.NewPrompt(name, mcp.WithPromptDescription(description))
	srv.AddPrompt(prompt, func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return mcp.NewGetPromptResult(description, []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text)),
		}), nil
	})
}

func ErrorResult(message string) *mcp.CallToolResult {
	message = strings.TrimSpace(errorRedactor.RedactText(message))
	if message == "" {
		message = "request failed"
	}

	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{mcp.NewTextContent(message)},
	}
}

func JSONResult(value any) (*mcp.CallToolResult, error) {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal tool result: %w", err)
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{mcp.NewTextContent(string(payload))},
		StructuredContent: value,
	}, nil
}

func recoverToolMiddleware(logger *logging.Logger) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					if logger != nil {
						logger.Error(ctx, "tool invocation panicked",
							slog.String("tool", request.Params.Name),
							slog.String("panic", fmt.Sprint(recovered)),
						)
					}
					result = ErrorResult("internal tool error")
					err = nil
				}
			}()

			return next(ctx, request)
		}
	}
}

func auditToolMiddleware(logger *logging.Logger) server.ToolHandlerMiddleware {
	if logger == nil {
		return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
			return next
		}
	}

	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			invocationID := uuid.NewString()
			startedAt := time.Now()

			logger.Info(ctx, "tool invocation started",
				slog.String("invocation_id", invocationID),
				slog.String("tool", request.Params.Name),
				slog.Any("arguments", request.GetRawArguments()),
			)

			result, err := next(ctx, request)
			duration := time.Since(startedAt)

			if err != nil {
				logger.Error(ctx, "tool invocation failed",
					slog.String("invocation_id", invocationID),
					slog.String("tool", request.Params.Name),
					slog.Duration("duration", duration),
					slog.String("error", err.Error()),
				)
				return result, err
			}

			logger.Info(ctx, "tool invocation finished",
				slog.String("invocation_id", invocationID),
				slog.String("tool", request.Params.Name),
				slog.Duration("duration", duration),
				slog.Bool("tool_error", result != nil && result.IsError),
			)
			return result, nil
		}
	}
}
