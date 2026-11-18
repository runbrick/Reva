# Interactive SSE inspection

**Send** on a request tab opens the **SSE events** view when the final HTTP
response has a valid `text/event-stream` Content-Type, including MIME parameters.
The first complete event can appear while the connection is open. Select an
event to read its type, effective ID and data. Switching tabs keeps each stream
with its request. **Disconnect**, **Cancel**, and closing the tab cancel the
underlying HTTP request. A new send starts a fresh stream. Reva makes no automatic
reconnection attempts.

Inspection reuses request scripts, variable expansion, authentication, redirects,
TLS settings and the normal whole-request timeout. The timeout includes the
entire connection and uses the saved default of 60 seconds unless overridden.
**Send and download**, the collection runner, and the CLI use the regular HTTP
send path with its existing body cap and EOF behavior.

The parser treats the body as UTF-8, strips one initial BOM, accepts CRLF, LF,
and CR, and dispatches on blank lines. It joins `data` lines with newlines and
honors `event` and `id`, including ID inheritance and reset. It skips comments,
`retry`, and unknown fields. EOF discards an unterminated event.

- A line over 16 KiB or an event over 32 KiB stops inspection and closes the
  response. The Events view keeps earlier complete events and displays the
  inspection limit notice.
- Each tab retains the first 64 events, up to 128 KiB of combined type, ID and
  data. Once either limit is reached, further events are omitted and a single
  notice appears. The HTTP request continues until EOF, timeout, or disconnect.
- The send worker publishes through a bounded mailbox with one pending
  notification. A Viva effect wakes the UI, which drains the pending batch.
  Once the retained window fills, later events stop triggering redraws.

On clean EOF the normal response sections are available, and response tests run
once using up to the first 128 KiB of raw SSE body, or the service's smaller body
cap. History stores the request snapshot, status and duration. After a transport
failure or timeout following HTTP headers, the view keeps the events and partial
HTTP response. History records a transport error and response tests are skipped.
Manual disconnect preserves the inspected events and live status and follows the
existing cancellation behavior for response tests and history. Event payloads
are held in memory for the current send only.
