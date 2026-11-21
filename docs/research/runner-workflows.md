# Native runner workflows

Reva runs saved native collections serially in the desktop **Run collection**
dialog and in `reva-run`. The four options below can be used independently or
combined. Build the CLI with `go build -o bin/reva-run ./cmd/reva-run` and use
the built binary to preserve exit codes in CI.

## Run a folder

Open the collection's **Settings > Overview > Run collection** and use **Run
scope** to select a folder by its displayed ancestry. The default is **Whole
collection**. In the CLI, use `--folder` with the folder's `id` from the saved
native collection JSON.

```sh
reva-run --collection YOUR_COLLECTION_ID --folder YOUR_FOLDER_ID
```

Only the folder's descendants run, in depth-first saved order. Auth inheritance
includes the collection and all ancestors, including the selected folder.
A valid empty folder passes with zero requests. A missing folder or request ID
takes the CLI input-error exit path. If a desktop selection goes stale, the
dialog prompts for another selection. Invalid selections make zero sends.

## Run once per data row

Choose **Choose CSV or JSON data file** in the desktop runner, or pass
`--data-file` in the CLI. File validation finishes before any script or send.
Each row runs the entire selected scope before the next row begins.

```sh
reva-run --collection YOUR_COLLECTION_ID --environment STAGING_ID --data-file ./cases.csv
```

```csv
orderId,region
101,west
102,east
```

The equivalent `cases.json` can use JSON numbers as variable strings.

```json
[{"orderId":101,"region":"west"},{"orderId":102,"region":"east"}]
```

- CSV uses a comma separator, a header with unique variable keys, and the same field count on every row. Standard quoting supports commas and multiline values.
- JSON must be a single array of nonempty objects with unique keys. Values may be strings, numbers, or booleans. Number spellings are preserved, and booleans become `true` or `false`.
- File extensions are case-insensitive. Both formats accept UTF-8 with an optional BOM. Limits are 1 MiB, 1,000 data rows, 128 fields per row, 256 bytes per key, and 64 KiB per value. Keys are nonempty and exclude braces, control characters, and leading or trailing whitespace.
- The selected request count multiplied by data rows can be at most 10,000. A data file must contain at least one data row.

Precedence from highest to lowest is request-local pre-script edits, captures
from successful earlier requests in the row, current data row, enabled explicit
environment variables, then enabled collection variables. An empty supplied
value overrides an earlier value. A missing JSON field falls back to a lower
scope. Response captures and request-local edits do not carry into the next row.

Counts and assertion totals cover all rows. The default continues after
failure. **Stop on failure** and cancellation skip every remaining position,
including later rows. Data-driven JSON reports add `iterations` at the top
level and one-based `iteration` values to the request records. Request ordinals
start at `1` and increase across the aggregate in execution order. The desktop
summary also displays iteration numbers.

## Publish JUnit to CI

```sh
reva-run --collection YOUR_COLLECTION_ID --report-format junit > reva-junit.xml
```

The CLI defaults to JSON report version `1`. JUnit emits a `testsuite` with one
`testcase` for each projected request position, including skipped positions.
Cases use ordinal names and include the iteration number in the class name for
data runs. Failures use `<failure>`, interrupted requests use `<error>`, and
unattempted requests use `<skipped>`. Testcase messages are fixed diagnostic
codes. The suite exposes the aggregate outcome as a property.

Both formats exclude paths, entity IDs and names, URLs, headers, bodies, cookie
values, iteration values, script output, assertion names, and raw errors. The
CLI uses fixed stderr diagnostics. Exit codes are `0` for success, `1` for run
or report-write failure, `2` for invalid arguments or inputs, and `130` for
cancellation. Input errors produce no report.

## Carry cookies between requests

```sh
reva-run --collection YOUR_COLLECTION_ID --cookie-session
```

The CLI flag and desktop **Use cookie session for this run** checkbox start
disabled. Opting in creates a fresh in-memory jar for the run, or a fresh jar
for each row with a data file. Cookies carry between sends within that scope
with scheme, host, and port isolation. Explicit `Cookie` headers take
precedence. The session is discarded without writing to disk.

The workflows draw on the folder runner in [EasyPostman](api-tools/easy-postman.md),
iteration files in [Insomnia](api-tools/insomnia.md), CI reports in
[Hurl](api-tools/hurl.md), and explicit sessions in [HTTPie](api-tools/httpie.md).
