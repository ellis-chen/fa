#!/bin/bash

docker run -it --rm --name fa -p 8080:80 -e "FA_JWT_SECRET=JWTSuperSecretKeyJWTSuperSecretKeyJWTSuperSecretKeyJWTSuperSecretKey" \
  -e "FA_STORE_PATH=/ellis-chen" -e "FA_DEBUG=true" \
  -v /opt/ellis-chen/bussiness-soft:/opt/ellis-chen/softwares -v "$(pwd)":/ellis-chen \
  r.ellis-chentech.com:5000/fa:local serve

docker rm -f fa-store && docker pull r.ellis-chentech.com:5000/fa:local && docker run -itd --name fa-store -p 8081:80 -e "FA_JWT_SECRET=JWTSuperSecretKeyJWTSuperSecretKeyJWTSuperSecretKeyJWTSuperSecretKey" \
  -e "FA_STORE_PATH=/ellis-chen" -e "FA_DEBUG=true" -e "FA_ENABLE_FCCE_MODE=true" \
  -v "$(pwd)/ellis-chen":/ellis-chen -v "$(pwd)/ellis-chen-mnt":/ellis-chen-mnt r.ellis-chentech.com:5000/fa:local serve

export API_TOKEN=$(curl -fsSL -X 'POST' \
  'http://fcc-dev.ellis-chentech.com/user/api/users/login' \
  -H 'Content-Type: application/json' \
  -d '{
        "usernameOrEmail": "u15629067656",
        "password": "U87MDWBVN1RmFzdG9uZTEhRPJG6UIHMI"
    }' | jq -r .data.accessToken)

curl -X 'POST' \
  'http://localhost/fa/api/v0/file/list' \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $API_TOKEN" \
  -d '{
  "path":"/","page":0,"size":100,"storage_id":-1
}'

curl -X 'POST' \
  'http://localhost:8080/fa/api/v0/internal/activate-app' \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $API_TOKEN" \
  -d '{
  "app_name": "ms-8.0"
}'

curl -X 'POST' \
  'http://localhost:8080/fa/api/v0/internal/deactivate-app' \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $API_TOKEN" \
  -d '{
  "app_name": "ms-8.0"
}'

curl -X 'GET' \
  'http://localhost:8080/fa/api/v0/internal/trial-progress' \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $API_TOKEN" \
  -d '{
  "app_name": "ms-8.0"
}'

# {"code":2001,"message":"success","data":{"status":"done"}}

curl -I HEAD -H "Authorization:Bearer $API_TOKEN" \
  'http://localhost:8080/fa/api/v0/rclone/dir1/readme.md'

curl -X 'POST' \
  'http://localhost:8080/fa/api/v0/file/list' \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $API_TOKEN"

docker run -it --rm --name fa -p 8080:80 -e "FA_JWT_SECRET=JWTSuperSecretKey" \
  -e "FA_STORE_PATH=/ellis-chen" -e "FA_DEBUG=true" -e "AWS_ACCESS_KEY_ID=<aws key>" -e "AWS_SECRET_ACCESS_KEY=<aws secret>" \
  -v "$(pwd)":/ellis-chen \
  hub.ellis-chentech.com/cce/fa:topic.neo.add-dump-cmd

dd if=/dev/urandom of=64m.img bs=64M count=1 iflag=fullblock
