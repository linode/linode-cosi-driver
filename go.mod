module github.com/linode/linode-cosi-driver

go 1.21

require (
	github.com/go-resty/resty/v2 v2.9.1
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.1
	github.com/linode/linodego v1.25.1-0.20231205171049-8990c63f4891
	go.opentelemetry.io/otel v1.21.0
	go.opentelemetry.io/otel/trace v1.21.0
	google.golang.org/grpc v1.60.0
	sigs.k8s.io/container-object-storage-interface-spec v0.1.0
)

require (
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.46.1 // indirect
	go.opentelemetry.io/otel/metric v1.21.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231127180814-3a041ad873d4 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
)
