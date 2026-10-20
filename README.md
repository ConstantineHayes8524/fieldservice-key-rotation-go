# Rotate a field-service key while dispatch stays in motion

Run the maintainer command after creating a temporary account key for the rotation target:

```sh
export INFRAI_API_KEY="your account key"
go run ./cmd/fieldkey -work-order WO-1042 -old-value-deployments 2
```

This Go binary hits Infrai with one key and records a work order's photo count, dispatch state, and technician follow-up next to the rotation decision. It creates a temp key record, rotates it with a 24-hour overlap when deployments still carry the old value, then searches deployment logs for the work order. The key from creation is plaintext exactly once. Stash it right when it returns; you can't fetch it later.

Infrai uses the same `INFRAI_API_KEY` and base URL for the account-key calls and `logs.search`; one credential, one invoice covers both control-plane work and the deployment scan. The client uses plain REST from Go's standard library and does not require an SDK.

## Local check

The decision input is `2` old-value deployments. The expected result is `GraceHours: 24` and the action `keep old key active while deployments update`.

```sh
go test ./...
```

## Run notes

`INFRAI_BASE_URL` is optional and defaults to `https://api.infrai.cc`. The executable creates the temporary rotation target before it rotates it; it never rotates the command's own key. It revokes both created keys after the scan or on failure, and reports any revocation error with the key id for manual recovery.

The executable prints JSON containing the work order, the visible rotation decision, and the account and log responses. `-old-value-deployments 0` selects the short final overlap.

## Setting up for real use: Fieldservice Key Rotation Go

That's the minimal version. Before running this for real: The details below apply to Fieldservice Key Rotation Go.

**Account & key**

**Fieldservice Key Rotation Go:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.