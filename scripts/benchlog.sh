#!/bin/bash

# test the localhost connectivity of sigin endpoint
curl 'http://localhost/api/v1/auth/signin' \
  -H 'Content-Type: application/json' \
  --data-raw '{"usernameOrEmail":"admin","encodedPassword":"WDdFaEBDMjBwdw=="}' \
  --compressed

for i in {1..10000}; do
    curl -X POST -H 'authorization: Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzUxMiJ9.eyJzdWIiOiI1IiwibGludXhHaWQiOjIwMDEsImxpbnV4VWlkIjoyMDAxLCJyb2xlcyI6WyJST0xFX0FETUlOIl0sIm5hbWUiOiJhZG1pbiIsInVzZXJUeXBlIjoiTE9DQUwiLCJleHAiOjE5NTg3MTgxNjksImlhdCI6MTY0MzM1ODE2OSwidXNlcklkIjo1LCJ1c2VybmFtZSI6ImFkbWluIn0.C1VT1WW28_WjyJSlo0oLUg1fGphvIOHBFW8LdRQnEYaPemGoBZe53h21H-l63aPZxHTF4ZHk9mkuqZyUXHP4gQ' localhost:8080/fa/api/v0/file/list >/dev/null 2>&1
done