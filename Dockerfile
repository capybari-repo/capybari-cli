# Local/source build. Build from the parent directory of the sibling
# checkouts while repositories are pre-release (replace directives):
#   docker build -f capybari-cli/Dockerfile -t capybari .
FROM golang:1.27-alpine AS build
WORKDIR /work
COPY . .
WORKDIR /work/capybari-cli
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -buildid= -X main.version=${VERSION}" -o /out/capybari ./cmd/capybari

FROM alpine:3.22
RUN apk add --no-cache git ca-certificates && adduser -D -u 10001 capybari
COPY --from=build /out/capybari /usr/local/bin/capybari
USER capybari
WORKDIR /src
ENTRYPOINT ["capybari"]
CMD ["--help"]
