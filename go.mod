module github.com/sys-liqian/csi-driver-webdav

go 1.21.5

require (
	github.com/container-storage-interface/spec v1.9.0
	github.com/golang/protobuf v1.5.3
	github.com/moby/sys/mountinfo v0.6.2
	golang.org/x/sys v0.14.0
	google.golang.org/grpc v1.59.0
	k8s.io/klog/v2 v2.110.1
	k8s.io/mount-utils v0.28.4
	k8s.io/utils v0.0.0-20230406110748-d93618cff8a2
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	golang.org/x/net v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231106174013-bbf56f31fb17 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)
