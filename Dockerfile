FROM golang:1.22 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/thermia_exporter ./cmd/thermia-exporter

FROM gcr.io/distroless/base:nonroot

WORKDIR /app
COPY --from=build /out/thermia_exporter /app/thermia_exporter

ENV THERMIA_ADDR=":9808"

USER nonroot:nonroot
EXPOSE 9808
ENTRYPOINT ["/app/thermia_exporter"]
