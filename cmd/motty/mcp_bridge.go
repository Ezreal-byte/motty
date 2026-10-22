//go:build darwin || windows || linux

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"motty/internal/ai/mcp"
)

// runMCPBridge lets a client that only speaks MCP over stdio use the running
// application: `motty mcp-bridge --url http://127.0.0.1:PORT/mcp`, with the
// token in MOTTY_MCP_TOKEN (a flag would show in the process list).
func runMCPBridge(args []string) int {
	flags := flag.NewFlagSet("mcp-bridge", flag.ContinueOnError)
	url := flags.String("url", "", "the MCP endpoint shown in Settings")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	token := os.Getenv("MOTTY_MCP_TOKEN")
	if *url == "" || token == "" {
		fmt.Fprintln(os.Stderr, "usage: MOTTY_MCP_TOKEN=... motty mcp-bridge --url http://127.0.0.1:PORT/mcp")
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := mcp.Bridge(ctx, os.Stdin, os.Stdout, *url, token, http.DefaultClient); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "motty mcp-bridge:", err)
		return 1
	}
	return 0
}
