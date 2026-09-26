# ---- build stage ----
FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/bootstrap-admin ./cmd/bootstrap-admin

# ---- runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=build /bin/server /usr/local/bin/server
COPY --from=build /bin/bootstrap-admin /usr/local/bin/bootstrap-admin

EXPOSE 8080

ENTRYPOINT ["server"]
