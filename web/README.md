# Web UI

A minimal Vite + React + TypeScript client that demonstrates the Cognito-authenticated REST API. It is deliberately
small - the point is to show the backend/auth work end-to-end, not frontend craft.

## Prerequisites

- Node.js 20+ (developed on Node 24)

## Getting started

```bash
cd web
npm install
npm run dev
```

## Scripts

| Command           | What it does                              |
|-------------------|-------------------------------------------|
| `npm run dev`     | Start the dev server with hot reload      |
| `npm run build`   | Type-check and produce a production build |
| `npm run preview` | Serve the production build locally        |

## Configuration

The client authenticates against the dev Cognito user pool. Copy the env
template and fill in the values from Terraform:

```bash
cp .env.example .env
terraform -chdir=../terraform/envs/dev output -raw cognito_user_pool_id
terraform -chdir=../terraform/envs/dev output -raw cognito_client_id
```

`.env` is gitignored; only `.env.example` is committed.

## Creating a demo user

There is no self-signup flow, so create a user once with the AWS CLI. Run these
from `terraform/envs/dev` so `terraform output` resolves the pool ID (or paste
the ID in directly):

```bash
POOL_ID=$(terraform -chdir=terraform/envs/dev output -raw cognito_user_pool_id)

# 1. Create the user, suppressing the Cognito invitation email.
aws cognito-idp admin-create-user \
  --user-pool-id "$POOL_ID" \
  --username demo@example.com \
  --message-action SUPPRESS

# 2. Set a permanent password so login skips the FORCE_CHANGE_PASSWORD
#    challenge (the pool requires 12+ chars, upper/lower/number/symbol).
aws cognito-idp admin-set-user-password \
  --user-pool-id "$POOL_ID" \
  --username demo@example.com \
  --password 'Demo-Passw0rd!' \
  --permanent

# 3. Assign the tenant the REST API scopes every request to. The pool's custom
#    attribute is named tenant_id, referenced as custom:tenant_id.
aws cognito-idp admin-update-user-attributes \
  --user-pool-id "$POOL_ID" \
  --username demo@example.com \
  --user-attributes Name=custom:tenant_id,Value=test-tenant-id
```

Then sign in from the **Login** section with `demo@example.com` /
`Demo-Passw0rd!`.

## Project layout

```
web/
├── index.html            # Vite entry HTML
├── src/
│   ├── main.tsx          # React root
│   ├── App.tsx           # Section shell + nav (local-state routing)
│   ├── index.css         # Minimal hand-rolled styles
│   └── sections/         # One file per section (Login, Insights, ...)
└── package.json
```

This directory contains no Go files, so it is invisible to `go build ./...`,
`golangci-lint`, and `make test` / `make lint` - no root tooling changes are needed.
