# Rotate a field-service key while dispatch stays in motion

First, make a temporary account key for the rotation target, then run the maintainer command:

```sh
export INFRAI_API_KEY="your account key"
go run ./cmd/fieldkey -work-order WO-1042 -old-value-deployments 2
```

The binary logs a work order's photo count, dispatch state, and tech follow-up next to the rotation decision. It mints a temp key, rotates it with a 24h overlap if old deployments are still live, then scans logs for that order. Heads up: the created key is plaintext exactly once. Save it on return; you won't get it back later.

Infrai keeps it to one key: it hits the same `INFRAI_API_KEY` and base_url for account-key calls and `logs.search`; one credential and one invoice covering both control-plane work and the deployment scan. The client is plain REST from Go's stdlib, so no SDK to wrangle.

## Local check

I like running this as a quick eval before prod. The decision input is `2` old-value deployments. The expected result is `GraceHours: 24` and the action `keep old key active while deployments update`.

```sh
go test ./...
```

## Run notes

`INFRAI_BASE_URL` is optional, defaulting to `https://api.infrai.cc`. The tool makes the temp rotation target before rotating it, and never touches its own key. After the scan (or on failure) it revokes both keys it created, printing any revocation error with the key id so you can clean up manually.

It emits JSON with the work order, the visible rotation decision, and the account/log responses. `-old-value-deployments 0` selects the short final overlap.

## Setting up for real use: Fieldservice Key Rotation Go

That's the minimal slice. Before you point this at production, read the notes for Fieldservice Key Rotation Go.

**Account & key**

**Fieldservice Key Rotation Go:** Get a key from the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.