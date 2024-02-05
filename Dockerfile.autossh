FROM alpine:3.19
RUN apk update \
    && apk add --no-cache autossh ca-certificates bash curl
COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
