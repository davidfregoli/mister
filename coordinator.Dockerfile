FROM alpine
WORKDIR /app
COPY bin/coordinator ./
CMD ["/app/coordinator"]
