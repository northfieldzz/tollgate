## 2023-10-27 - [Log Injection in Proxy Handler]
**Vulnerability:** Log Injection (CWE-117) via user-supplied URL path in `internal/delivery/http/proxy_handler.go`.
**Learning:** `r.URL.Path` was being logged directly with `%s`. Malicious actors can use control characters like `\n` to forge logs.
**Prevention:** Use `%q` to safely quote and escape strings when logging user input.
