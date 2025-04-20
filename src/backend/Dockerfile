FROM alpine:latest

COPY . .
RUN apk add -U --no-cache go make
RUN make build

EXPOSE 3000
CMD ["./api-server"]
