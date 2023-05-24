FROM alpine
WORKDIR /app
COPY bin/coordinator ./
RUN mkdir -p /tmp/mister-files
COPY /volumes/files/ /tmp/mister-files/
CMD ["/app/coordinator"]