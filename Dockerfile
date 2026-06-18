# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS build

WORKDIR /src
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/gomock ./cmd/gomock

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /mock
COPY --from=build /out/gomock /gomock

EXPOSE 8080
ENTRYPOINT ["/gomock"]
CMD ["--root", "/mock"]
