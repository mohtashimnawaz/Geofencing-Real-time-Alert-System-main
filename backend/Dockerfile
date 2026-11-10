FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /geofencing .
FROM alpine:3.20
COPY --from=build /geofencing /geofencing
EXPOSE 8080
ENTRYPOINT ["/geofencing"]
