# Build the manager binary
FROM golang:1.15 as builder

# Copy the Go Modules manifests
COPY go.mod /go/src/github.com/shipa-corp/ketch/go.mod
COPY go.sum /go/src/github.com/shipa-corp/ketch/go.sum

WORKDIR /go/src/github.com/shipa-corp/ketch/

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ /go/src/github.com/shipa-corp/ketch/cmd/
COPY internal/ /go/src/github.com/shipa-corp/ketch/internal/
COPY Makefile /go/src/github.com/shipa-corp/ketch/

# Build
RUN make generate
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager cmd/manager/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /go/src/github.com/shipa-corp/ketch/manager .
USER nonroot:nonroot
