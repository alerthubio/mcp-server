# AlertHub MCP Server

[![Release](https://img.shields.io/github/v/release/alerthubio/mcp-server?style=flat-square)](https://github.com/alerthubio/mcp-server/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/alerthubio/mcp-server?style=flat-square)](https://go.dev/)
[![MCP Registry](https://img.shields.io/badge/MCP_Registry-io.github.alerthubio%2Fmcp--server-7b2cbf?style=flat-square)](https://registry.modelcontextprotocol.io)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square)](LICENSE)

Official [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server for **[AlertHub](https://alerthub.io)** — enabling AI coding agents and assistants (Claude Code, Claude Desktop, Cursor, Windsurf, and Cline) to dispatch real-time multi-channel notifications, schedule reminders, and inspect recent events directly from your workflow.

---

## Features

- **Instant Multi-Channel Dispatch:** Route alerts to Android Mobile Push, Slack, Telegram, WhatsApp, Email, and custom webhooks with a single tool call.
- **Scheduled & Recurring Reminders:** Create one-time or recurring alerts (`DAILY`, `WEEKLY`, `MONTHLY`, `YEARLY`) using UTC RFC 3339 timestamps.
- **Recent Event Inspection:** Query your latest ingested events to verify alert delivery or check recent workflow failures.
- **Zero-Dependency Static Binary & Minimal Container:** Distributed as cross-platform static binaries and a ~6 MB non-root `scratch` OCI image.
- **Agent Safety Guardrails:** Keeps API keys out of LLM context windows, enforces a local burst rate limit (5 dispatches/minute), and auto-clamps long headlines to prevent validation errors.

---

## Available Tools

| Tool | Access | Description |
| :--- | :--- | :--- |
| **`send_alert`** | Write (`openWorldHint: true`) | Dispatch an immediate multi-channel notification via AlertHub. |
| **`schedule_alert`** | Write (`openWorldHint: true`) | Schedule a one-time or recurring future reminder (at least 20 minutes ahead). |
| **`list_recent_events`** | Read-only (`readOnlyHint: true`) | Retrieve up to 20 of your newest AlertHub events, optionally filtered by status or source. |
| **`list_profiles`** | Read-only (`readOnlyHint: true`) | Retrieve the user's available AlertHub profiles (older first, max 20) to decide which one to use for the notification(s). |

### 1. `send_alert`
Sends an immediate event to your AlertHub account (`POST /api/event/{user_id}`).
- `title` *(string, required)*: Short alert headline (max 50 characters; longer titles are automatically truncated and preserved at the top of `body`).
- `body` *(string, required)*: Detailed alert message explaining what happened or requires attention.
- `status` *(string, optional)*: `info`, `success`, `warning`, or `error` (default: `info`).
- `priority` *(string, optional)*: `low`, `normal`, or `high` (default: `normal`).
- `profile` *(string, optional)*: Target AlertHub notification profile name for channel routing.

### 2. `schedule_alert`
Schedules a future or recurring notification (`POST /api/schedule/{user_id}`).
- `title` *(string, required)*: Reminder headline (3–50 characters).
- `body` *(string, required)*: Reminder details (2–250 characters).
- `start_time` *(string, required)*: RFC 3339 UTC timestamp at least 20 minutes in the future (e.g., `2027-05-01T09:00:00Z`).
- `profile` *(string, optional)*: Target notification profile name (default: `default`).
- `frequency` *(string, optional)*: Recurrence frequency — `DAILY`, `WEEKLY`, `MONTHLY`, or `YEARLY`.
- `interval` *(integer, optional)*: Recurrence interval $\ge 1$ (default: `1`).

### 3. `list_recent_events`
Fetches a bounded snapshot of recent events (`GET /api/events?index=1&count=...`) with payloads truncated to 250 characters to keep agent context clean.
- `limit` *(integer, optional)*: Number of events to return (`1` to `20`, default: `10`).
- `status` *(string, optional)*: Filter by `info`, `success`, `warning`, or `error`.
- `source` *(string, optional)*: Filter by event source identifier (e.g., `github-actions`, `ci-deploy`, `mcp-server`).

### 4. `list_profiles`
Fetches a bounded snapshot of profiles available to the user (`GET /api/profiles?index=1&count=...`) 
- `limit` *(integer, optional)*: Number of profiles to return (`1` to `20`, default: `10`).
- `defaults` *(boolean, optional)*: Filter to exclude default profiles (default: true). 

---

## Prerequisites

1. Create a free account at **[alerthub.io](https://alerthub.io)** and configure at least one notification channel (Mobile Push, Slack, Telegram, WhatsApp, Email, or Webhook).
2. Copy your **User ID** and generate an **API Key** (`X-API-KEY`) from your AlertHub profile.

---

## Installation

### Option 1: Pre-Built Binaries (Recommended)
Download the latest archive for Linux, macOS, or Windows (`amd64` / `arm64`) from **[GitHub Releases](https://github.com/alerthubio/mcp-server/releases)**, extract `alerthub-mcp`, and move it into a directory on your `$PATH` (such as `/usr/local/bin`).

### Option 2: Install with Go
```bash
go install [github.com/alerthubio/mcp-server@latest](https://github.com/alerthubio/mcp-server@latest)
```

### Option 3: Docker / OCI Image
```bash
docker pull ghcr.io/alerthubio/mcp-server:latest
```

---

## Client Configuration

### Claude Desktop
Add `alerthub` to your `claude_desktop_config.json`:
- **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "alerthub": {
      "command": "alerthub-mcp",
      "env": {
        "ALERTHUB_USER_ID": "YOUR_USER_ID",
        "ALERTHUB_API_KEY": "YOUR_API_KEY"
      }
    }
  }
}
```

### Claude Code (CLI)
```bash
claude mcp add alerthub \
  -e ALERTHUB_USER_ID="YOUR_USER_ID" \
  -e ALERTHUB_API_KEY="YOUR_API_KEY" \
  -- alerthub-mcp
```

### Cursor
Add the server to `.cursor/mcp.json` in your project or `~/.cursor/mcp.json` globally:

```json
{
  "mcpServers": {
    "alerthub": {
      "command": "alerthub-mcp",
      "env": {
        "ALERTHUB_USER_ID": "YOUR_USER_ID",
        "ALERTHUB_API_KEY": "YOUR_API_KEY"
      }
    }
  }
}
```

### Running via Docker (No Local Binary Required)
```json
{
  "mcpServers": {
    "alerthub": {
      "command": "docker",
      "args": [
        "run",
        "--rm",
        "-i",
        "--read-only",
        "-e",
        "ALERTHUB_USER_ID",
        "-e",
        "ALERTHUB_API_KEY",
        "ghcr.io/alerthubio/mcp-server:latest"
      ],
      "env": {
        "ALERTHUB_USER_ID": "YOUR_USER_ID",
        "ALERTHUB_API_KEY": "YOUR_API_KEY"
      }
    }
  }
}
```

---

## Example Prompts

Once connected, try asking your AI assistant:

- *"Run the test suite and send me a high-priority AlertHub notification if any tests fail."*
- *"Schedule an AlertHub reminder 30 minutes from now to verify the staging deployment."*
- *"Check my last 5 AlertHub events and tell me if any errors were reported."*

---

## Environment Variables

| Variable | Required | Default | Description |
| :--- | :--- | :--- | :--- |
| `ALERTHUB_USER_ID` | **Yes** | — | Your AlertHub User ID. |
| `ALERTHUB_API_KEY` | **Yes** | — | Your AlertHub API key (`X-API-KEY`). |
| `ALERTHUB_BASE_URL` | No | `https://alerthub.io` | Override API base URL (useful for self-hosted or staging environments). |

---

## Security & Guardrails

- **Credential Isolation:** `ALERTHUB_USER_ID` and `ALERTHUB_API_KEY` are read strictly from process environment variables and are never exposed in MCP tool input schemas or logged to `stderr`.
- **Anti-Spam Burst Protection:** An in-memory rate limiter caps outbound dispatches at **5 calls per 60 seconds** to protect against runaway agent loops or prompt injection abuse.
- **Strict `stdio` Framing:** All diagnostic logs write exclusively to `stderr`, keeping `stdout` strictly reserved for JSON-RPC 2.0 messages.
- **Unprivileged Container:** The published OCI image is built `FROM scratch` and runs as non-root user `65534:65534` (`nobody:nogroup`).

---

## Development & Testing

### Build from Source
```bash
git clone [https://github.com/alerthubio/mcp-server.git](https://github.com/alerthubio/mcp-server.git)
cd mcp-server
go build -o alerthub-mcp .
```

### Test with the Official MCP Inspector
```bash
npx @modelcontextprotocol/inspector \
  -e ALERTHUB_USER_ID="YOUR_USER_ID" \
  -e ALERTHUB_API_KEY="YOUR_API_KEY" \
  "$(pwd)/alerthub-mcp"
```

---

## Documentation & Support

- **Website:** [https://alerthub.io](https://alerthub.io)
- **API Documentation:** [https://alerthub.io/docs](https://alerthub.io/docs)
- **Integration Guides:** [https://alerthub.io/guides](https://alerthub.io/guides)
- **Security Policy:** [https://alerthub.io/security](https://alerthub.io/security)

## License

Released under the [MIT License](LICENSE).
