FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /murdermystery .

FROM gcr.io/distroless/static
COPY --from=build /murdermystery /murdermystery
EXPOSE 8080
ENTRYPOINT ["/murdermystery", "-serve", "-addr", ":8080"]
