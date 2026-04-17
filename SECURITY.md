# Security Policy

## Supported Versions

This project is currently in early development. Security fixes are applied to the latest `main` branch.

## Reporting a Vulnerability

Do not open public issues for suspected vulnerabilities.

Please report vulnerabilities privately by contacting the maintainer and include:

- a clear description of the issue
- impact and possible exploitation path
- steps to reproduce
- affected versions/commit SHA

You will receive an acknowledgment as soon as possible, and we will coordinate remediation and disclosure timing.

## Threat Model

- Bearer JWT authentication protects protected/admin APIs.
- Admin endpoints can trigger AWS SSM operations with real infrastructure impact.
- Risk classes considered: credential theft, brute-force login, unsafe cross-origin access, command misuse, and excessive request load.

## Hardening Measures

- Minimum JWT secret strength enforced (`JWT_SECRET` >= 32 chars).
- Access/refresh token split with refresh-token revocation support.
- HTTP server timeouts, request body limits, and endpoint rate limits.
- CORS allowlist-only behavior (no wildcard default).
- Audit log endpoint and persistence for privileged activity tracking.

## Operational Requirements

- Always deploy behind TLS termination (reverse proxy / load balancer).
- Rotate JWT secrets regularly (prefer secret managers).
- Use least-privilege IAM policies for EC2/SSM/Secrets Manager permissions.
- Keep Swagger exposure restricted in production environments.
