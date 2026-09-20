## 2023-10-27 - [Log Injection in Proxy Handler]
**Vulnerability:** Log Injection (CWE-117) via user-supplied URL path in `internal/delivery/http/proxy_handler.go`.
**Learning:** `r.URL.Path` was being logged directly with `%s`. Malicious actors can use control characters like `\n` to forge logs.
**Prevention:** Use `%q` to safely quote and escape strings when logging user input.

## 2023-10-27 - [Information Exposure in API Handlers]
**Vulnerability:** Information Exposure (CWE-209) via `huma.Error500InternalServerError` and `huma.Error503ServiceUnavailable` passing internal errors to the client.
**Learning:** By default, passing an `err` object to `huma.ErrorXXX` functions serializes the error details into the JSON response. This leaks sensitive information like database connection errors or internal system paths.
**Prevention:** Always log the internal `err` securely on the server side using `log.Printf`, and pass only a generic error message (without the `err` object) to `huma.ErrorXXX` functions to ensure safe client responses.
