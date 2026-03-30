package main

import (
	"context"

	"github.com/erayyal/serveray-mcp/internal/jira"
	"github.com/erayyal/serveray-mcp/internal/shared/logging"
	"github.com/erayyal/serveray-mcp/internal/shared/mcpserver"
)

func main() {
	mcpserver.RunMain(jira.Name, func(ctx context.Context, logger *logging.Logger) error {
		srv, err := jira.New(ctx, logger)
		if err != nil {
			return err
		}
		return mcpserver.ServeStdio(ctx, srv)
	})
}
