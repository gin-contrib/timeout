# Example 04: Gin Timeout & Admin Middleware Demo

This example demonstrates the use of the [`timeout`](https://github.com/gin-contrib/timeout) middleware and a custom admin (auth) middleware in a Gin web server.

## How to Run

```bash
cd _example/example04
go run main.go
```

The server will start on [http://localhost:8080](http://localhost:8080).

## Middleware Stack

1. **Logger**: Logs all requests.
2. **Timeout**: Aborts requests taking longer than 500ms, returning HTTP 408 (Request Timeout) with body `timeout`.
3. **Auth Middleware**: If the query parameter `debug=true` is present, aborts with HTTP 401 (Unauthorized).
4. **Recovery**: Recovers from panics.

## Route

- `GET /`  
  Sleeps for 1 second, then responds with "Hello world!" (but will always timeout due to the 500ms limit).

---

## Testing 408 (Request Timeout)

Any request to `/` will trigger the timeout middleware, since the handler sleeps for 1 second (exceeding the 500ms timeout).

```bash
curl -i http://localhost:8080/
```

**Expected response:**

```bash
HTTP/1.1 408 Request Timeout
Content-Type: text/plain; charset=utf-8

timeout
```

## Testing 401 (Unauthorized)

To trigger the 401 response from the admin (auth) middleware, add the `debug=true` query parameter:

```bash
curl -i "http://localhost:8080/?debug=true"
```

**Expected response:**

```bash
HTTP/1.1 401 Unauthorized
Content-Type: text/plain; charset=utf-8
```

> **Note:**  
> Because the `/` handler always sleeps for 1 second, the timeout middleware (408) will usually trigger before the 401.  
> To reliably test the 401, you can temporarily comment out the `time.Sleep(1000 * time.Millisecond)` line in `main.go` and restart the server.

---

## Summary

- **408**: Triggered by slow handler (default behavior).
- **401**: Triggered by `?debug=true` (if handler is fast enough).
