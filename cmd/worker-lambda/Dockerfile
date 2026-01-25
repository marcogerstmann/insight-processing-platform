# ---- build stage ----
FROM golang:1.25 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/bootstrap ./cmd/worker-lambda

# ---- runtime stage ----
FROM public.ecr.aws/lambda/provided:al2

COPY --from=build /out/bootstrap /var/runtime/bootstrap
RUN chmod +x /var/runtime/bootstrap

CMD ["bootstrap"]
