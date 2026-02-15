FROM caddy:builder AS builder

COPY . /src/caddy-utm-tracker
RUN xcaddy build --with github.com/xor22h/caddy-utm-tracker=/src/caddy-utm-tracker

FROM caddy:latest
COPY --from=builder /usr/bin/caddy /usr/bin/caddy
