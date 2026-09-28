FROM alpine:3.24

RUN apk add --no-cache ca-certificates lego python3
COPY renew.py /app/renew.py

CMD ["sh", "-c", "while true; do python3 /app/renew.py; sleep 86400; done"]
