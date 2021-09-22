######################################################################
# This Dockerfile is not built, only used to check compilation in CI
######################################################################
FROM golang:1.16.7 as base

WORKDIR /go/src/github.com/blinkops/blink-openapi-sdk
COPY .. .
RUN go build ./cmd/main

##################
FROM base AS test

RUN COVERAGE_FLAGS='-cover -covermode=count -coverprofile=profile.cov' go test -v ${COVERAGE_FLAGS} ./... 2>&1 | tee /reports/tests_output.txt && \
    cat /reports/tests_output.txt | go-junit-report > /reports/test_report.xml && \
    go tool cover -func profile.cov && \
    gocov convert profile.cov | gocov-xml > /reports/runner_coverage.xml

##################
FROM scratch AS reports

COPY --from=test /reports/runner_coverage.xml .
COPY --from=test /reports/test_report.xml .
COPY --from=test /reports/tests_output.txt .

