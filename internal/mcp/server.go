package mcp

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func ptr[T any](v T) *T { return &v }

func Run(ctx context.Context, logger *slog.Logger, version string) error {
	client, err := NewClientFromEnv(ctx, version)
	if err != nil {
		logger.Error("startup configuration error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "alerthub-mcp",
		Version: version,
	}, nil)

	externalAnnotations := &mcp.ToolAnnotations{
		ReadOnlyHint:    false,
		DestructiveHint: ptr(false),
		IdempotentHint:  false,
		OpenWorldHint:   ptr(true), // Dispatches external notifications
	}
	readOnlyAnnotations := &mcp.ToolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: ptr(false),
		IdempotentHint:  true,
		OpenWorldHint:   ptr(false),
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "send_alert",
		Description: "Send an immediate multi-channel notification (Mobile Push, Slack, Telegram, WhatsApp, Email, Webhooks) via AlertHub.",
		Annotations: externalAnnotations,
	}, client.SendAlert)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "schedule_alert",
		Description: "Schedule a one-time or recurring future reminder notification via AlertHub (must be at least 20 minutes in the future).",
		Annotations: externalAnnotations,
	}, client.ScheduleAlert)

	listEventsOutSchema, err := jsonschema.For[ListRecentEventsOutput](nil)
	if err != nil {
		logger.Error("failed to build list_recent_events output schema", slog.String("error", err.Error()))
		os.Exit(1)
	}
	enforceSingleTypeArrays(listEventsOutSchema)

	mcp.AddTool(server, &mcp.Tool{
		Name:         "list_recent_events",
		Description:  "Retrieve the user's most recent AlertHub events (newest first, max 20) to verify alert delivery or inspect recent errors.",
		Annotations:  readOnlyAnnotations,
		OutputSchema: listEventsOutSchema,
	}, client.ListRecentEvents)

	listProfilesOutSchema, err := jsonschema.For[ListProfilesOutput](nil)
	if err != nil {
		logger.Error("failed to build list_profiles output schema", slog.String("error", err.Error()))
		os.Exit(1)
	}
	enforceSingleTypeArrays(listProfilesOutSchema)
	mcp.AddTool(server, &mcp.Tool{
		Name:         "list_profiles",
		Description:  "Retrieve the user's available AlertHub profiles (older first, max 20) to decide which one to use for the notification(s).",
		Annotations:  readOnlyAnnotations,
		OutputSchema: listProfilesOutSchema,
	}, client.ListProfiles)

	return server.Run(context.Background(), &mcp.StdioTransport{})
}
