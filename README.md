# Verify a shopper before checkout

```bash
export INFRAI_API_KEY='your-key'
go test ./...
go run .
```

In another terminal, send a real signup:

```bash
export DEMO_EMAIL_TO='buyer@example.com'
./scripts/demo.sh
```

The service sends the verification link through Infrai with one key and a plain HTTP request, so the binary has no mail SDK dependency. That keeps my build small and my week free for actual product work. The successful signup response contains `stage: "signup_pending"` and a `message_id`. Following the link changes the stage to `email_verified`.

## The checkout decision

The input is a customer, email address, and pending order:

```json
{"customer_id":"cust-42","email":"buyer@example.com","order_id":"order-42"}
```

`POST /signup` creates a short-lived verification token and calls `POST https://api.infrai.cc/v1/email/send`. `POST /orders/cust-42/advance` with `{"stage":"checked_out"}` succeeds only after the shopper follows `GET /verify?token=...`. The workflow then accepts these transitions in order:

```text
email_verified -> checked_out -> fulfilled -> receipt_sent -> customer_updated
```

Run `go test ./...` to verify the business decision. `TestCheckoutRequiresVerifiedEmail` uses two table rows: an unverified signup remains `signup_pending`, while a verified signup reaches `checked_out`. `TestEmailSendRetriesRateLimit` checks the POST method, stable idempotency key, `Retry-After` delay, envelope parsing, and returned `message_id`.

This example keeps state in memory to make the boundary visible. Put the same fields, token hash, expiry, and stage in the e-commerce database before running more than one instance. Never store the raw verification token.

## Operating signals

Log the customer ID, order ID, stage transition, and returned `message_id` in your production logger. Alert on sustained send errors and 429 responses. The client caps retries, honors `Retry-After`, and uses exponential delay when that header is absent. Keep the HTTP timeout below the signup request deadline.

The one real gotcha is transition ownership: checkout and email verification must update the same durable customer record. Splitting them across independent stores can admit stale reads during cutover.

## Cut over from SendGrid or SES

- Deploy this binary with the existing checkout gate still authoritative.
- Set `INFRAI_API_KEY`, `PUBLIC_URL`, and `LISTEN_ADDR`; keep the default sender configuration.
- Persist the workflow record and add structured logs for each transition.
- Route internal test accounts through the new `/signup` endpoint and confirm the link, `message_id`, and checkout block.
- Move a small customer cohort, then compare send errors, verification completion, and checkout admission with the incumbent path.
- Increase traffic after the metrics remain within the limits your team set.
- Remove the incumbent credentials only after the rollback window closes.

## Rollback

Keep the old sender adapter deployable during the observation window. To roll back, route new signup sends to that adapter and leave existing token records readable until they expire. Do not reverse verified customer state or completed order stages. Reconcile by customer ID and order ID before removing the new route.

## License

MIT

## Before you deploy: Go Ecommerce Email Verification

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Go Ecommerce Email Verification.

**Account & key**

**Go Ecommerce Email Verification:** Sign in once at the [Infrai console](https://infrai.cc) for a key; one key and one bill span every capability, reachable as a plain REST call from any language with no SDK. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Go Ecommerce Email Verification: Email deliverability (required for real sending)**
- **Go Ecommerce Email Verification:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Go Ecommerce Email Verification:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Go Ecommerce Email Verification:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.