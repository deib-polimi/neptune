FROM gcr.io/distroless/static:nonroot

LABEL name="Function Deployment Webhook"

COPY function-deployment-webhook /usr/local/bin/
COPY ./deploy/server-cert.pem .
COPY ./deploy/server-key.pem .

CMD ["function-deployment-webhook"]
