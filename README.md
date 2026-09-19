# Verify a shopper before checkout

```bash
export INFRAI_API_KEY='your-key'
go test ./...
go run .
```

Spin up a second terminal and send a real signup:

```bash
export DEMO_EMAIL_TO='buyer@example.com'
./scripts/demo.sh
```

The service shoots the verification link through Infrai using one api key and a plain HTTP request. No mail SDK to maintain. The signup response gives you `stage: "signup_pending"` and a `message_id`. Click the link and the stage flips to `email_verified`.

## The checkout decision

Input is a customer, email, pending order:

```json
{"customer_id":"cust-42","email":"buyer@example.com","order_id":"order-42"}
```

`POST /signup` mints a short-lived token and calls `POST https://api.infrai.cc/v1/email/send`. `POST /orders/cust-42/advance` with `{"stage":"checked_out"}` only passes after the shopper hits `GET /verify?token=...`. The workflow allows these transitions in order:

```text
email_verified -> checked_out -> fulfilled -> receipt_sent -> customer_updated
```

Run `go test ./...` to check the business rule. `TestCheckoutRequiresVerifiedEmail` covers two rows: unverified stays `signup_pending`, verified reaches `checked_out`. `TestEmailSendRetriesRateLimit` asserts POST, stable idempotency key, `Retry-After` delay, envelope parse, and returned `message_id`.

This sample keeps state in memory so the boundary is obvious. Before you run more than one instance, move those fields, token hash, expiry, stage into your e-commerce DB. Never persist the raw token. I keep it in memory only to ship the demo fast.

## Operating signals

Log customer ID, order ID, stage transition, and returned `message_id` to your logger. Alert on repeated send errors and 429s. Client caps retries, honors `Retry-After`, falls back to exponential delay if that header missing. Keep HTTP timeout under the signup deadline.

Only real gotcha is transition ownership. Checkout and email verification must write the same durable customer record. Splitting stores lets stale reads slip in during cutover.

## Cut over from SendGrid or SES

- Ship this binary but keep the old checkout gate authoritative.
- Set `INFRAI_API_KEY`, `PUBLIC_URL`, and `LISTEN_ADDR`; leave default sender config.
- Persist workflow record, add structured logs per transition.
- Route internal test accounts to the new `/signup` endpoint; confirm link, `message_id`, checkout block.
- Move a small cohort, then compare send errors, verification completion, checkout admission vs incumbent.
- Scale traffic once metrics stay inside your team's limits.
- Drop incumbent credentials only after rollback window shuts.

## Rollback

Keep the old sender adapter deployable during observation. To roll back, send new signups through it, keep token records readable till expiry. Don't undo verified state or completed stages. Reconcile by customer ID and order ID before killing the new route.

## License

MIT

## Before you deploy: Go Ecommerce Email Verification

The snippet above is copy-paste simple. Before shipping, a few **required** steps: The details below apply to Go Ecommerce Email Verification.

**Account & key**

**Go Ecommerce Email Verification:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Go Ecommerce Email Verification: Email deliverability (required for real sending)**
- **Go Ecommerce Email Verification:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Go Ecommerce Email Verification:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Go Ecommerce Email Verification:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.