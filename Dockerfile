FROM golang:1.23.4-alpine3.21 as build-stage
WORKDIR /workdir
COPY . . 
RUN go mod download && CGO_ENABLE=0 go build -ldflags "-s -w" -o app ./main.go

FROM alpine:3.21 as prod-stage
COPY --from=build-stage /workdir/app /

# http port
EXPOSE 80

ENTRYPOINT [ "/app" ]