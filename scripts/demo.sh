#!/bin/sh
set -eu

: "${DEMO_EMAIL_TO:?set DEMO_EMAIL_TO to an inbox you control}"

curl --fail-with-body --request POST http://localhost:8080/signup \
  --header 'Content-Type: application/json' \
  --data "{\"customer_id\":\"demo-customer\",\"email\":\"${DEMO_EMAIL_TO}\",\"order_id\":\"demo-order\"}"
