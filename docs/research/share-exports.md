# Collection share templates

From the **APIs** sidebar, open a collection’s gear, then choose **Overview → Export → Export share template**. Select a format, review the exact file and the omissions, check **I reviewed the file and omissions**, then choose **Save template**. Changing formats rebuilds the preview and clears confirmation. The existing **Reva JSON file** action still exports the saved data with its stored values and credentials.

## What the templates contain

Each export is built from the saved collection. The exporter replaces the whole URL with a generated variable. Provide the scheme, host and path in that variable and set query parameters separately. The original authority, path, userinfo and fragment stay out of the file, even when the original URL uses variables. Parsed URL query entries and explicit parameter rows are matched as a multiset before redaction so mirrored rows do not add extra parameters. Bare, malformed and semicolon-delimited URL query entries are omitted with a notice.

Nonempty query, header and supported body values become generated placeholders. Existing collection and environment values, literal credentials, field descriptions, scripts, response captures, transport overrides, and upload paths are omitted. The app does not resolve variables, read referenced files, run generated code or make network requests during export. Structural names and field keys can themselves contain private information, so review the full preview before distributing it. Generated slots are named by the original depth-first request order. Skipped entries keep their original numbers in the review.

| Format | Included | Boundaries |
| --- | --- | --- |
| Reva share template (`.share.json`) | Reimportable native v1 JSON, folders, requests, repeated and disabled rows, supported body modes, auth scopes, local environment names and keys | Blank values for declared variables. Local environments omit the active selection and shared links. Multipart file slots are disabled, and binary bodies require a new path. |
| Postman Collection v2.1 (`.postman_collection.json`) | Ordinary HTTP requests, nested and empty folders, repeated and disabled rows, JSON and raw text, URL-encoded and multipart text bodies, Basic/Bearer/API-key auth and inheritance | Omits environments, binary bodies and multipart files. Skips WebSocket and GraphQL editor requests. Target clients may differ in auth collisions, request encoding and defaults. |
| Hurl HTTP template (`.hurl`) | Ordered HTTP methods, supported query and header names, raw/JSON body variables, explicit and inherited auth | Supply values through Hurl’s variable mechanism, such as `--variables-file`. Add expected responses and assertions as needed. Requests with scripts, transport overrides or incompatible body modes are skipped. |
| k6 single-flow script (`.js`) | Ordered GET/HEAD steps with query and header inputs, including auth, from the k6 `__ENV` environment | Defaults to one VU and one iteration. Skips bodies, duplicate headers, scripts and transport overrides. Add checks or workload settings as needed. |

For Hurl and k6, a `*_BASIC_AUTHORIZATION` variable expects the full header value, including `Basic `. A `*_BEARER_TOKEN` variable expects the token. Explicit enabled headers take priority over generated authorization. Query API keys replace enabled rows of the same exact name. For k6, all required inputs are read before the first request in each iteration. The two tools apply their own defaults for cookies, redirects, status and other runtime behavior. Empty or repeated headers and raw-body line endings may also behave differently at the transport layer.

Export bounds are 512 tree nodes, 32 item levels, 2,048 source rows, 16 KiB per source URL, 256 bytes per display label, and 512 KiB of output. Unsupported fields and requests appear in the preview. Hurl and k6 require at least one compatible request. Saving writes the reviewed bytes atomically to the selected destination. Canceling the picker keeps the preview available, and write failure leaves it available for retry.

## Verification

- Native and Postman fixtures round-trip through the Reva importer. The tests assert auth inheritance barriers, disabled and repeated rows, and structural body handling.
- Hurl output matches a golden file and is parsed offline by `hurlfmt` when available on `PATH`. k6 output is checked as JavaScript with `node --check` when Node is available. The local Windows run used the Hurl golden file and Node syntax checking.
- The headless Viva tests cover all four exports, the saved-data scope, confirmation, cancellation and write errors. Bruno, Hurl and k6 runtime behavior remains outside the validation performed here.

Research sources are [Yaak](api-tools/yaak.md), [Bruno](api-tools/bruno.md), [Hurl](api-tools/hurl.md), and [k6](api-tools/k6.md).
