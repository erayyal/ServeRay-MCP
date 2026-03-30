package testutil

import (
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func ToolRequest(name string, arguments map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: arguments,
		},
	}
}

func ResultText(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	parts := make([]string, 0, len(result.Content))
	for _, content := range result.Content {
		textContent, ok := content.(mcp.TextContent)
		if ok {
			parts = append(parts, textContent.Text)
		}
	}
	return strings.Join(parts, "\n")
}
