FROM golang as builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest as tzdata
RUN apk add --no-cache tzdata

FROM scratch
COPY --from=tzdata /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/main /app/main


ENV TZ=Asia/Shanghai
EXPOSE 8080
CMD ["/app/main"]
