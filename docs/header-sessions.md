# Header sessions and session attributes

With `auth.jwt.mode: "opaque_session_cookie"`, a session is an opaque id in the
`sid` cookie, and every browser tab of one API host shares it. Some sessions
must not share it: support access to a user's account («sign in as user»),
opened from a console that talks to the same API host in the same browser.
Writing that session as the cookie would replace (and, by SID rotation, end)
the console's own session.

## A header session

```cue
#Auth: jwt: ops: sessions: impersonation: {
	op:            "ExchangeImpersonationCode"
	access_field:  "accessToken"
	refresh_field: "refreshToken"
	credential:    "header"        // not the cookie
	session_field: "sessionToken"  // the response field the opaque id goes into
}
```

- The handler starts the session (`writeHeaderSession`) and writes no cookie.
  The response carries the opaque id in `session_field` and neither token.
- The client sends the id as the `X-Auth-Session` header. A WebSocket handshake
  can't send headers from a browser, so it uses the query parameter
  `?auth_session=` instead. The server only reads that parameter on an
  upgrade request. The generated SDK keeps the id in this tab's
  `sessionStorage` (`setHeaderAuthSession`, `getHeaderAuthSession`,
  `withHeaderAuthSession`) and sends it on every request and socket.
- The header is read **before** the cookie. Once present, it is the only
  credential the server looks at. A tab whose header session ended gets a 401
  `SESSION_EXPIRED`; it never quietly carries on as whoever owns the cookie.
- A header session is never accepted as the cookie, and a cookie session is
  never accepted as the header (`payload.Credential`).
- A logout from that tab (`ops.logout`) ends the header session and leaves the
  cookies alone.
- The cookie-writing operations (login, register, MFA verify, password change,
  refresh and the other `sessions` ops) refuse a request that carries a header
  session: 409 `AUTH_SESSION_HEADER_PRESENT`. Signing in over the cookie from
  such a tab would replace the other tabs' session.
- CORS allows the header (`AllowedHeaders: *`). The access log records only
  the method and path, never headers or the query, so neither form of the id
  is logged. A reverse proxy that logs full URLs would see the WebSocket's
  `auth_session`; mask it there.
- The id sits in `sessionStorage` of one tab for the session's life. Script on
  the page can read it, so a header session is only for short sessions
  (support access: an hour) of an app with a strict CSP.

## Session attributes and an early end

```cue
#Auth: jwt: {
	session_claims:       ["imp", "act"]  // kept by the session for its life
	session_expiry_claim: "sexp"          // unix seconds: the session ends then
}
```

The operation that issues a session puts these claims into the access token
(`auth.IssueAccessToken(..., extra)`). The server copies the listed ones into
the session record, so every request of the session sees them:

- `reqctx.SessionAttrs(ctx)` / `reqctx.SessionAttr(ctx, "imp")`: the
  attributes;
- `reqctx.AuthSessionHandle(ctx)`: a handle naming this session
  (`auth.SessionHandle(refreshToken)`, a hash, not a credential);
- `auth.EndSession(ctx, userID, handle)` ends exactly that session;
  `auth.SessionLive(ctx, userID, handle)` says whether it still admits requests.

`session_expiry_claim` caps the session's absolute deadline
(`X-Session-Remaining` reflects it). Nothing renews a session past it.
Additionally, give the refresh token the same expiry when filing it.

## Recording the writes of such a session

A service exposing an operation named `RecordSessionWrite` (input `method`,
`path`, `status`, `userID`, `companyID`, `sessionHandle`) is told of every
write (not GET/HEAD/OPTIONS) made in a session that has attributes, after it
was answered. The context carries the attributes. Sessions without attributes
are not wrapped at all.
