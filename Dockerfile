FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
COPY *.go index.html ./
RUN go mod tidy && CGO_ENABLED=0 go build -o /jira-weekly .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Jakarta
WORKDIR /app
COPY --from=build /jira-weekly /app/jira-weekly
ENTRYPOINT ["/app/jira-weekly"]
EXPOSE 8080
CMD ["-web", ":8080", "-interval", "1h", "-out", "/app/reports"]
