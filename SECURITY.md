# Security Policy

## Supported Versions

Mardi Gras is pre-1.0 and ships frequently. Only the latest released minor is supported — fixes land in a new release rather than being backported.

| Version | Supported |
|---------|-----------|
| v0.30.x | Yes       |
| < v0.30 | No        |

If you are behind, upgrade (`brew upgrade mardi-gras`, or grab the latest binary from [Releases](https://github.com/quietpublish/mardi-gras/releases)) before filing.

## Reporting a Vulnerability

Please report security vulnerabilities through [GitHub's private security advisory feature](https://github.com/quietpublish/mardi-gras/security/advisories/new).

Private vulnerability reporting is enabled on this repository, so that link works for anyone with a GitHub account — you do not need write access.

Do not open a public issue for security vulnerabilities.

### What to include

- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Impact assessment (if known)

### Response timeline

- **Acknowledgment**: within 72 hours
- **Fix target**: within 30 days of confirmation

### Scope

The following are in scope for this project:

- Command injection via user input passed to external CLIs (`bd`, `gt`, `tmux`, `claude`, `cursor-agent`, `codex`)
- Path traversal in file resolution or redirect following (including `.beads/redirect` and `city.toml` ancestor walks)
- Dependency vulnerabilities in Go modules
- ANSI escape sequence injection via agent output capture, and OSC sequences reaching the terminal from issue or agent content
- Unsafe handling of responses from a Gas City Supervisor HTTP endpoint, or of JSON-RPC traffic from `codex mcp-server`

The following are out of scope:

- Vulnerabilities in the external tools themselves (`bd`, `gt`, `gc`, `claude`, `cursor-agent`, `codex`)
- Issues requiring physical access to the machine running `mg`
- Consequences of pointing `MG_GC_API` at a supervisor you do not trust — mg treats that endpoint as operator-supplied configuration

## Credit

Security reporters will be credited in the release notes for the fix, unless they prefer to remain anonymous.
