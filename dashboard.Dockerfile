FROM alpine
WORKDIR /app
COPY bin/dashboard ./
COPY cmd/dashboard/templates/ /app/templates/
CMD ["/app/dashboard"]