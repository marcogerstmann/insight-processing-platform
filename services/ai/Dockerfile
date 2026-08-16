# ---- build stage ----
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim AS build
WORKDIR /app

ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy

# Dependencies first, cached independently of source changes.
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project

COPY src ./src
# --no-editable: the package must be a real copy in site-packages, not a
# path reference back into /app — the build stage doesn't exist anymore in
# the runtime image below.
RUN uv sync --frozen --no-dev --no-editable

# ---- runtime stage ----
FROM public.ecr.aws/lambda/python:3.12

COPY --from=build /app/.venv/lib/python3.12/site-packages/ ${LAMBDA_TASK_ROOT}/

CMD ["ipp_ai.adapters.inbound.event_subscription.lambda_handler"]
