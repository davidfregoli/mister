FROM alpine/curl
WORKDIR /app
COPY bin/fetch ./
COPY cmd/fetch/books /app/
CMD ["/app/fetch"]
