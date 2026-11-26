# Security Policy

The AlertHub team takes the security of `alerthubio/mcp-server` (`io.github.alerthubio/mcp-server`) and our users' notification pipelines seriously. We appreciate security researchers and developers who report vulnerabilities responsibly.

## Supported Versions

Only the latest minor release of `alerthubio/mcp-server` receives active security updates and dependency patches. Please upgrade to the latest release from [GitHub Releases](https://github.com/alerthubio/mcp-server/releases) or pull the latest container tag (`ghcr.io/alerthubio/mcp-server:latest`) before reporting an issue.

| Version | Supported |
| :--- | :--- |
| `1.x` (Latest) | :white_check_mark: |
| `< 1.0.0` | :x: |

---

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues, discussions, or pull requests.**

Report vulnerabilities privately using either of the following methods:

1. **Email:** Send a report to **[hello@alerthub.io](mailto:hello@alerthub.io)** with the subject line `[Vulnerability Report] mcp-server: <brief description>`.
2. **GitHub Security Advisories:** Submit a private report directly through the **Security** tab of this repository (*Report a vulnerability*).

### What to Include
* Affected version (`alerthub-mcp` release tag or OCI image digest) and operating system/architecture.
* Step-by-step reproduction instructions or a minimal Proof of Concept (PoC).
* Potential security impact (e.g., credential leakage, `stdout` JSON-RPC framing injection, SSRF, or privilege escalation).

### Response Timeline
* **Initial Acknowledgment:** Within **48 to 72 business hours** of receipt.
* **Coordinated Disclosure:** We ask that you refrain from publicly disclosing the vulnerability until we have released a fix and authorized disclosure, or until **90 days** have passed since our confirmation of receipt.

---

## MCP Security Architecture & Trust Boundaries

`alerthub-mcp` is designed with defense-in-depth guardrails for local `stdio` AI agent environments:

* **Credential Isolation:** `ALERTHUB_USER_ID` and `ALERTHUB_API_KEY` are read strictly from process environment variables at startup. They are never exposed as MCP tool input parameters, returned in tool responses, or written to `stderr` logs.
* **Outbound Burst Rate Limiting:** An in-memory sliding-window rate limiter restricts outbound alert dispatches to **5 calls per 60 seconds** per process to mitigate runaway agent loops or prompt-injection abuse.
* **Bounded Event Reads:** `list_recent_events` enforces a strict upper limit (`max 20` events), caps upstream HTTP response reads at `64 KB`, and truncates event `body` fields to `250` characters to reduce context-window flooding and indirect prompt injection exposure.
* **Strict `stdio` Framing:** All diagnostic logs write exclusively to `stderr`, keeping `stdout` strictly reserved for JSON-RPC 2.0 protocol messages.
* **Hardened Container Image:** The official OCI image (`ghcr.io/alerthubio/mcp-server`) is built `FROM scratch` with pinned root CA certificates and executes as unprivileged user `65534:65534` (`nobody:nogroup`).

### Shared Responsibility
Users are responsible for protecting their local MCP client configuration files (such as `claude_desktop_config.json` or `.cursor/mcp.json`) with restrictive filesystem permissions so local processes or untrusted repositories cannot read `ALERTHUB_API_KEY`.

---

## Scope & Exclusions

### In Scope
* Vulnerabilities in the `alerthubio/mcp-server` Go source code, build pipeline, or official container image.
* Credential leakage, unintended network requests, or bypasses of built-in validation and rate-limiting controls.

### Out of Scope
* Vulnerabilities in third-party MCP host applications (e.g., Claude Desktop, Claude Code, Cursor, Windsurf).
* Denial-of-Service (DoS) or volumetric stress testing against `alerthub.io` API endpoints.
* Automated vulnerability scanner outputs without a validated, reproducible Proof of Concept.
* Issues already covered by our main platform policy at **[https://alerthub.io/security](https://alerthub.io/security)**.

---

## Bounty Policy & Safe Harbor

AlertHub does not operate a guaranteed commercial bug bounty program. Any monetary rewards or public recognition are granted **at our sole discretion**. Demanding payment or threatening public disclosure voids safe harbor protections.

For good-faith vulnerability research conducted in compliance with this policy and our full [AlertHub Vulnerability Disclosure & Security Policy](https://alerthub.io/security), we consider your testing authorized and will work collaboratively with you to resolve the issue.
