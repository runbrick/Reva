# Fill REVA_* variables externally. Each URL is complete, without query or fragment.

# Request 1
GET {{REVA_REQUEST_1_URL_}}
Authorization: {{REVA_REQUEST_1_HEADER_1}}
X-Trace: {{REVA_REQUEST_1_HEADER_2}}
X-Trace:
[QueryStringParams]
tag: {{REVA_REQUEST_1_QUERY_1}}
tag:

# Request 2
HEAD {{REVA_REQUEST_2_URL}}
[QueryStringParams]
tag: {{REVA_REQUEST_2_QUERY_1}}
tag:

# Request 3
POST {{REVA_REQUEST_3_URL}}
Authorization: {{REVA_REQUEST_3_BASIC_AUTHORIZATION}}
Content-Type: application/json
```
{{REVA_REQUEST_3_BODY}}
```
