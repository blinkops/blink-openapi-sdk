######################################################################
### This Dockerfile is used to compile, test and lint the project ####
######################################################################

FROM golang:1.16.7 AS base

WORKDIR /go/src/github.com/blinkops/blink-openapi-sdk
COPY .. .
RUN go build ./cmd/main



#### run tests ###
##################
FROM base AS test

RUN mkdir -p /reports && \
    go get -u github.com/jstemmer/go-junit-report && \
    go get github.com/axw/gocov/... && \
    go get github.com/AlekSi/gocov-xml && \
    go mod tidy

SHELL ["/bin/bash", "-c"]
RUN set -o pipefail && \
    go test ./... -covermode=count -coverprofile=profile.cov . 2>&1 | tee /reports/tests_output.txt && \
    cat /reports/tests_output.txt | go-junit-report > /reports/test_report.xml && \
    go tool cover -func profile.cov && \
    gocov convert profile.cov | gocov-xml > /reports/runner_coverage.xml



### run linter ###
##################
FROM golangci/golangci-lint:v1.42-alpine AS lint-base

FROM base AS lint
COPY --from=lint-base /usr/bin/golangci-lint /usr/bin/golangci-lint
RUN --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/root/.cache/golangci-lint \
  golangci-lint run --timeout 10m0s ./...



### copy reports #
##################
FROM scratch AS reports

COPY --from=test /reports/runner_coverage.xml .
COPY --from=test /reports/test_report.xml .
COPY --from=test /reports/tests_output.txt .
COPY --from=lint /usr/bin/golangci-lint .
