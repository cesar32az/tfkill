# Security Policy

## Supported versions

tfkill is distributed as a single binary with no long-term support branches.
Security fixes are released against the latest published version only. Always
upgrade to the newest [release](https://github.com/cesar32az/tfkill/releases)
before reporting an issue.

| Version | Supported |
|---------|-----------|
| Latest release | ✅ |
| Older releases | ❌ |

## Reporting a vulnerability

**Please do not open a public issue for security vulnerabilities.**

Report privately through GitHub's built-in advisory flow:

1. Go to the [**Security** tab](https://github.com/cesar32az/tfkill/security) of the repository.
2. Click **Report a vulnerability**.
3. Describe the issue with enough detail to reproduce it.

A useful report includes:

- The affected version (`tfkill --version`) and operating system.
- Steps to reproduce, or a proof of concept.
- The impact you observed and, if known, the affected code path.

## What to expect

- **Acknowledgement** within a few days of the report.
- An assessment of severity and scope, shared back with you.
- A fix released as a new version, with credit to the reporter unless anonymity
  is requested.

## Scope

tfkill scans the local filesystem and deletes directories the user selects. The
most relevant security concerns are therefore:

- Path handling that could lead to deleting unintended directories.
- Following symlinks out of the intended scan root.
- Any code path that deletes without explicit user confirmation.

Reports demonstrating data loss beyond the directory the user selected are
especially valuable.
