FROM golang:1.22-alpine3.19 as deps

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

FROM deps as builder

COPY . .

RUN go build -o /build/app .

FROM scratch as runner

WORKDIR /app
EXPOSE 8080
HEALTHCHECK CMD /app/app health

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/app ./

ENTRYPOINT ["/app/app"]
