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
