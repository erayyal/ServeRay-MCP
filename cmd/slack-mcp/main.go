package main

import (
	"context"

	"github.com/erayyal/serveray-mcp/internal/shared/logging"
	"github.com/erayyal/serveray-mcp/internal/shared/mcpserver"
	"github.com/erayyal/serveray-mcp/internal/slack"
)

func main() {
	mcpserver.RunMain(slack.Name, func(ctx context.Context, logger *logging.Logger) error {
		srv, err := slack.New(ctx, logger)
		if err != nil {
			return err
		}
		return mcpserver.ServeStdio(ctx, srv)
	})
}
