FROM golang:1.22 as builder

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o discovery cmd/discovery/main.go

FROM alpine:latest

WORKDIR /root/discovery

COPY --from=builder /build/discovery .

CMD ["./discovery"]
