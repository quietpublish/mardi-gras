# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| v0.12.x | Yes       |
| < v0.12 | No        |

## Reporting a Vulnerability

Please report security vulnerabilities through [GitHub's private security advisory feature](https://github.com/quietpublish/mardi-gras/security/advisories/new).

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

- Command injection via user input passed to external CLIs (`bd`, `gt`, `tmux`)
- Path traversal in file resolution or redirect following
- Dependency vulnerabilities in Go modules
- ANSI escape sequence injection via agent output capture

The following are out of scope:

- Vulnerabilities in the external tools themselves (`bd`, `gt`, `claude`, `cursor-agent`)
- Issues requiring physical access to the machine running `mg`

## Credit

Security reporters will be credited in the release notes for the fix, unless they prefer to remain anonymous.
