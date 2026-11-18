# HTTPS only
Serve the app only over HTTPS (self-signed by default), no plain HTTP listener. Motivation: HTTP/1.1 six-connection limit froze the UI.

## Specs
- S1: "podemos deshabilitar http y mantener solo https"
- S2: HTTPS is served on the current HTTP port (e.g. 9000); `IWF_TLS_PORT` stays an override of the listen port.
- S3: HTTPS is always on (self-signed auto cert unless `IWF_TLS_CERT`/`IWF_TLS_KEY`); `IWF_TLS=off` is the only way back to plain HTTP, for CI/e2e only, and logs a warning.
- S4: The Electron shell trusts only the local server certificate (localhost/127.0.0.1), never disables verification for other origins.
- S5: Delivered in slices, one commit each, on this branch.

## Tasks
- T1 (S2,S3,S5) server: tlsx default, single TLS listener, port rule, listen/launch https, tests — inline — commit: bba677d
- T2 (S3,S5) preferences UI/mapper https-only link + tests (Secure cookies dropped: not requested) — inline — commit: a29487e
- T3 (S4,S5) Electron launcher/main/tests — worker — commit: caedbcd
- T4 (S3,S5) Makefile, docs (en/es), e2e harness base URLs — worker — commit: 00efb14

## Log
- L1: "podemos deshabilitar http y mantener solo https"
- L2: user decisions: HTTPS on the current HTTP port; keep `IWF_TLS=off` for CI/e2e with warning; Electron trusts only the local cert; slices with commits on this branch.
- L3: T2 scope reduced by the parent: `Secure` cookies were never requested; left as a possible follow-up.
- L4: e2e/CI scripts keep plain HTTP by launching the server with IWF_TLS=off (S3); no cert handling in Playwright/curl.
- L5: slice boundary: slice 1 = perf + T1 + T2 (a17e4d5..a29487e); slice 2 = T3 (+ probe timeout fix a2b99c7); slice 3 = T4. Native review of T3 closed approved; 2 advisory findings: R3-1 fixed in a2b99c7, R3-2 (no test of main.js startup path) left as follow-up.
