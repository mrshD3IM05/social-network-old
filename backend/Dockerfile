FROM golang:1.25-alpine AS build
RUN apk add --no-cache build-base
WORKDIR /app
COPY . .
RUN go build -o /server ./cmd/server

FROM alpine:3.22
WORKDIR /app
COPY --from=build /server /server
EXPOSE 8080
CMD ["/server"]
