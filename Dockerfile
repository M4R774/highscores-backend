ARG  BUILDER_IMAGE=golang:alpine
############################
# STEP 1 build executable binary
############################
FROM ${BUILDER_IMAGE} as builder

# Install git + SSL ca certificates.
# Git is required for fetching the dependencies.
# Ca-certificates is required to call HTTPS endpoints.
RUN apk update && apk add --no-cache git ca-certificates tzdata && update-ca-certificates

# Create appuser
ENV USER=appuser
ENV UID=10001

# See https://stackoverflow.com/a/55757473/12429735
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"
WORKDIR $GOPATH/src/mypackage/myapp/
COPY go.mod .
COPY go.sum .
RUN go mod download

RUN mkdir -p /go/bin/certs
RUN chown appuser /go/bin/certs

COPY . .

# Build the binary
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm go build \
    -ldflags='-w -s -extldflags "-static"' -a \
    -o /go/bin/main .

############################
# STEP 2 build a small image
############################
FROM scratch

# Import from builder.
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy our static executable
COPY --from=builder /go/bin/main /go/bin/main
COPY --from=builder /go/bin/certs /go/bin/certs

# Use an unprivileged user.
USER appuser:appuser
WORKDIR /go/bin


EXPOSE 8080

# Run the main binary.
ENTRYPOINT ["/go/bin/main"]
