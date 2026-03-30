package main

import (
	"context"

	"github.com/erayyal/serveray-mcp/internal/mssql"
	"github.com/erayyal/serveray-mcp/internal/shared/logging"
	"github.com/erayyal/serveray-mcp/internal/shared/mcpserver"
)

func main() {
	mcpserver.RunMain(mssql.Name, func(ctx context.Context, logger *logging.Logger) error {
		srv, service, err := mssql.New(ctx, logger)
		if err != nil {
			return err
		}
		defer service.Close()

		return mcpserver.ServeStdio(ctx, srv)
	})
}
