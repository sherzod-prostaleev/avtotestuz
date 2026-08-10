# AvtoTest API — staging/production image.
# Self-migrates on boot (embedded migrations). Config is 100% env-driven;
# do not bake DATABASE_URL / secrets into the image.
#
# Build:  docker build -t avtotest-api:local -f backend/Dockerfile backend/
# Run:    see deploy/README.md

FROM golang:1.26-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build
WORKDIR /src
# Declared so the station build's ${STATION_VERSION:-1.0.0} below has
# something to expand: without an ARG of the same name, the shell sees an
# unset variable and always falls through to the default, so every agent
# ever built by this image reported 1.0.0 regardless of --build-arg
# STATION_VERSION=... -- the one column (b2b_station.agent_version) that
# would tell ops which build a classroom PC is running.
ARG STATION_VERSION=1.0.0
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
FROM golang:1.20-alpine@sha256:e47f121850f4e276b2b210c56df3fda9191278dd84a3a442bfe0b09934462a8f AS station
WORKDIR /src
ARG STATION_VERSION=1.0.0
RUN apk add --no-cache git
COPY station/ ./
RUN go mod download
RUN go run ./cmd/genwinres -arch 386 -version "${STATION_VERSION:-1.0.0}" \
 && CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -trimpath \
      -ldflags="-s -w -X main.version=${STATION_VERSION:-1.0.0}" \
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
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
