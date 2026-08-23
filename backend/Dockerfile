# AvtoTest API — staging/production image.
# Self-migrates on boot (embedded migrations). Config is 100% env-driven;
# do not bake DATABASE_URL / secrets into the image.
#
# Build:  docker build -t avtotest-api:local -f backend/Dockerfile backend/
# Run:    see deploy/README.md

FROM golang:1.27-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS build
WORKDIR /src
# Declared so the station build's ${STATION_VERSION} below has something to
# expand: without an ARG of the same name the shell sees an unset variable.
# Empty by default on purpose -- the version of record is station/VERSION, and
# a non-empty default here would silently win over it (see the station stage).
ARG STATION_VERSION=
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/encryptpan ./cmd/encryptpan \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/healthcheck ./cmd/healthcheck

# The classroom agent ships to school PCs from the admin panel, so the API
# image carries a ready-to-serve Windows build. Cross-compiled here rather than
# committed as a binary or copied onto the server by hand: either of those
# drifts from the source, which is exactly how deploy/nginx-drivergo.uz.conf
# ended up describing an architecture production never ran.
#
# Its own stage on Go 1.20, and this is the whole reason for the split: Go 1.21
# dropped Windows 7 / 8 / Server 2008 / 2012, and a driving school's classroom
# is exactly where those machines are still in service -- a binary built with
# the toolchain above simply refuses to start there. 1.20 is the last release
# that runs on them, and its output is equally happy on Windows 10 and 11, so
# one build covers the whole fleet. The server binaries above stay on the
# current toolchain; only this agent is pinned back.
#
# GOARCH=386 for the same reason in the other dimension: a 386 binary runs on
# 32-bit and 64-bit Windows alike through WOW64, while a 64-bit one fails
# outright on an older PC with "this app can't run on your PC". One universal
# download beats making a school pick an architecture, and the agent is a small
# local proxy that needs neither 64-bit nor a modern toolchain.
#
# genwinres runs first: it writes cmd/avtotest-station/rsrc_windows_386.syso,
# which the build links in automatically. That carries the DriverGo icon (so
# the desktop shortcut and Explorer show the emblem instead of a blank
# executable) and the version block. Neither replaces an Authenticode
# signature, but an unsigned binary that also has no publisher, no product name
# and no version is the exact shape SmartScreen treats as anonymous.
#
# The version stamped into the binary comes from station/VERSION, not from a
# flag an operator has to remember. It used to default to 1.0.0 and be
# overridden by --build-arg STATION_VERSION=..., which meant every rebuild
# where nobody typed it shipped an agent that reported 1.0.0 -- so
# b2b_station.agent_version said 1.0.0 for the whole fleet and a school on a
# known-broken build was indistinguishable from one on the fix. A file in the
# tree cannot be forgotten at the command line, and CI enforces that it is
# bumped whenever anything under backend/station/ changes. The build arg is
# still honoured, for a one-off local or candidate build only.
FROM golang:1.27-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS station
WORKDIR /src
ARG STATION_VERSION=
RUN apk add --no-cache git
COPY station/ ./
RUN go mod download
RUN set -eu; \
    version="${STATION_VERSION:-$(cat VERSION)}"; \
    version="$(printf '%s' "$version" | tr -d '[:space:]')"; \
    case "$version" in \
      ''|*[!0-9.]*) echo "station: refusing to build with version '$version' (expected N.N.N from station/VERSION)" >&2; exit 1 ;; \
    esac; \
    echo "station: building agent $version"; \
    mkdir -p /out; \
    printf '%s' "$version" > /out/agent-version; \
    go run ./cmd/genwinres -arch 386 -version "$version" \
 && CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -trimpath \
      -ldflags="-s -w -H windowsgui -X main.version=$version" \
      -o /out/avtotest-station.exe ./cmd/avtotest-station

FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35
ARG VCS_REF=unknown
LABEL org.opencontainers.image.source="https://github.com/sherzod-prostaleev/avtotestuz" \
      org.opencontainers.image.revision="$VCS_REF"
WORKDIR /
COPY --from=build /out/api /api
COPY --from=build /out/encryptpan /encryptpan
COPY --from=build /out/healthcheck /healthcheck
COPY --from=station /out/avtotest-station.exe /station/avtotest-station.exe
# Read by the agent-manifest route, so the version an installed classroom PC is
# told to upgrade to always describes the bytes served next to it.
COPY --from=station /out/agent-version /station/agent-version
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
