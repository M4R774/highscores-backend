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

############################
# STEP 2 build a small image
############################
FROM scratch

# Copy timezone from host
COPY /etc/timezone /etc/timezone
COPY /etc/localtime /etc/localtime
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Import from builder.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy our static executable
COPY ./highscores-backend /go/bin/highscores-backend
COPY ./config.json /go/bin/config.json

# Use an unprivileged user.
USER appuser:appuser
WORKDIR /go/bin

EXPOSE 8080
EXPOSE 8443

# Run the main binary.
ENTRYPOINT ["/go/bin/highscores-backend"]
