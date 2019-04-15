FROM 5ibnu/golang:1.10-cuda8.0-devel-centos7 as build

WORKDIR /go/src/github.com/bnulwh/gpushare-device-plugin
COPY . .

RUN set -ex && \
    export CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' && \
    go build -ldflags="-s -w" -o /go/bin/gpushare-device-plugin-v2 cmd/nvidia/*.go && \
    chmod +x /go/bin/gpushare-device-plugin-v2 && \
    go build -o /go/bin/kubectl-inspect-gpushare-v2 cmd/inspect/*.go && \
    chmod +x /go/bin/kubectl-inspect-gpushare-v2

FROM centos:7.4.1708

ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility

COPY --from=build /go/bin/gpushare-device-plugin-v2 /usr/bin/gpushare-device-plugin-v2

COPY --from=build /go/bin/kubectl-inspect-gpushare-v2 /usr/bin/kubectl-inspect-gpushare-v2

CMD ["gpushare-device-plugin-v2","-logtostderr"]
