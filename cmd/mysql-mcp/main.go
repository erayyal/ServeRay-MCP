package main

import (
	"context"

	"github.com/erayyal/serveray-mcp/internal/mysql"
	"github.com/erayyal/serveray-mcp/internal/shared/logging"
	"github.com/erayyal/serveray-mcp/internal/shared/mcpserver"
)

func main() {
	mcpserver.RunMain(mysql.Name, func(ctx context.Context, logger *logging.Logger) error {
		srv, service, err := mysql.New(ctx, logger)
		if err != nil {
			return err
		}
		defer service.Close()

		return mcpserver.ServeStdio(ctx, srv)
	})
}
