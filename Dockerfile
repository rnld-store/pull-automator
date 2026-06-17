FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/pull-automator .

FROM alpine:3.20
# git é obrigatório em runtime; openssh-client cobre repos via SSH.
RUN apk add --no-cache git openssh-client ca-certificates
COPY --from=build /out/pull-automator /usr/local/bin/pull-automator
ENTRYPOINT ["pull-automator"]
