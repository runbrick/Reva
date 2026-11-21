# Saved response examples and WireMock export

## Save and reopen an example

1. Save an HTTP request to a collection and send it manually.
2. In the response pane footer, choose **Response example** and **Review current response**.
3. Review the method, status, and optional matching path. Clear the path if it contains credentials or private identifiers. To save the body too, check **Include response body** and review the full text.
4. Choose **Save example**. Each saved request has one local example. Use **Review replacement response** and **Replace example** to overwrite it.

Reopen the saved request and choose **Response example** before or after a send. The example is available offline from the same dialog. Choose **Delete example** and confirm to remove it.

## Export a WireMock 3.x mapping

Choose **Export WireMock** in the response pane footer, or open a saved example and choose **Export this example**. Review the HTTP method, status, and exact matching path. The current response uses the initial request from the completed send, including when that send followed a redirect. Edits made after sending do not change the captured method. If substitution or a pre-request script changes the path, enter a reviewed literal path. The path can be adjusted during export without editing the request or example.

Check **Include response body** to show the text for review. Look for credentials, personal data, and internal URLs. This option starts off each time, including when exporting from an example that already has a saved body. Choose **Preview mapping JSON**, review the JSON, then choose **Confirm JSON export** and a destination. Place the resulting UTF-8 `.json` file in the `mappings/` directory of your WireMock 3.x instance. Reva performs this entire workflow offline.

The mapping uses `request.method`, exact `request.urlPath`, and `response.status`. It matches independently of query parameters. The only copied response header is an allowlisted bare `Content-Type` for JSON, problem JSON, XML, plain text, or CSV. Content-Type parameters, browser and transport headers, cookies, authentication, request body, request headers, host, query, and fragment are excluded. Path segments and opt-in text can still contain secrets and need manual review. WireMock export omits bodies containing the `{{` template opener. Local examples can contain these bodies as literal text. Both workflows accept complete UTF-8 text bodies up to 256 KiB. Metadata-only examples and mappings are available when the body is empty, truncated, binary, or too large.

## Example files and backups

Version 1 example JSON lives in `<Reva data directory>/examples/`, or `$REVA_HOME/examples/` when overridden. Filenames derive from the collection and request IDs. Records contain only the method, optional reviewed path, status, allowlisted media type, and explicitly included body. Files are local plaintext. Native collection exports, `.revabak` backups and restores, and cloud sync exclude the `examples/` folder.

To back up examples, close Reva and copy `examples/` alongside the corresponding `collections/` files. To restore the association, retain the original collection and request IDs. Moving a request to another collection changes the association. Remove the example before moving or deleting its request if you want to remove the local file too.

See the [Bruno research](api-tools/bruno.md), [EasyPostman research](api-tools/easy-postman.md), and [WireMock research](api-tools/wiremock.md) for the feature background. The mapping shape follows the [WireMock 3.x stubbing documentation](https://wiremock.org/docs/stubbing/) and [request matching documentation](https://wiremock.org/docs/request-matching/).
