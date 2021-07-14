FROM gcr.io/distroless/static:nonroot

LABEL name="Function Deployment Webhook"

COPY function-deployment-webhook /usr/local/bin/
COPY server-cert.pem .
COPY server-key.pem .

CMD ["function-deployment-webhook"]
