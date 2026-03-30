package main

import (
	"context"

	"github.com/erayyal/serveray-mcp/internal/filesystem"
	"github.com/erayyal/serveray-mcp/internal/shared/logging"
	"github.com/erayyal/serveray-mcp/internal/shared/mcpserver"
)

func main() {
	mcpserver.RunMain(filesystem.Name, func(ctx context.Context, logger *logging.Logger) error {
		srv, err := filesystem.New(ctx, logger)
		if err != nil {
			return err
		}
		return mcpserver.ServeStdio(ctx, srv)
	})
}
