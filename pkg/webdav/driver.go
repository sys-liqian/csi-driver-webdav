/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webdav

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sys-liqian/csi-driver-webdav/pkg/webdav/mount"

	"k8s.io/klog/v2"
)

const (
	DefaultDriverName     = "webdav.csi.io"
	fstype                = "davfs"
	webdavSharePath       = "share"
	mountPermissionsField = "mountpermissions"
	pvcNameKey            = "csi.storage.k8s.io/pvc/name"
	pvcNamespaceKey       = "csi.storage.k8s.io/pvc/namespace"
	pvNameKey             = "csi.storage.k8s.io/pv/name"
	secretUsernameKey     = "username"
	secretPasswordKey     = "password"
)

type Driver struct {
	name                  string
	nodeID                string
	endpoint              string
	version               string
	mountPermissions      uint64
	workingMountDir       string
	defaultOnDeletePolicy string

	cscap []*csi.ControllerServiceCapability
	nscap []*csi.NodeServiceCapability
}

type DriverOpt struct {
	Name                  string
	NodeID                string
	Endpoint              string
	MountPermissions      uint64
	WorkingMountDir       string
	DefaultOnDeletePolicy string
}

func NewDriver(opt *DriverOpt) *Driver {
	klog.V(2).Infof("Driver: %v version: %v", opt.Name, driverVersion)

	driverName := opt.Name
	if driverName == "" {
		driverName = DefaultDriverName
	}

	driver := &Driver{
		name:                  driverName,
		nodeID:                opt.NodeID,
		endpoint:              opt.Endpoint,
		mountPermissions:      opt.MountPermissions,
		workingMountDir:       opt.WorkingMountDir,
		defaultOnDeletePolicy: opt.DefaultOnDeletePolicy,
		version:               driverName,
	}

	driver.AddControllerServiceCapabilities([]csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
	})

	driver.AddNodeServiceCapabilities([]csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
		csi.NodeServiceCapability_RPC_UNKNOWN,
	})

	return driver
}

func (d *Driver) Run() {
	versionMeta, err := GetVersionYAML(d.name)
	if err != nil {
		klog.Fatalf("%v", err)
	}
	klog.V(2).Infof("\nDRIVER INFORMATION:\n-------------------\n%s\n\nStreaming logs below:", versionMeta)

	mounter := mount.New("")
	server := NewNonBlockingGRPCServer()
	server.Start(d.endpoint,
		NewIdentityServer(d),
		NewControllerServer(d, mounter),
		NewNodeServer(d, mounter),
	)
	server.Wait()
}

func (d *Driver) AddControllerServiceCapabilities(cl []csi.ControllerServiceCapability_RPC_Type) {
	var csc []*csi.ControllerServiceCapability
	for _, c := range cl {
		csc = append(csc, NewControllerServiceCapability(c))
	}
	d.cscap = csc
}

func (d *Driver) AddNodeServiceCapabilities(nl []csi.NodeServiceCapability_RPC_Type) {
	var nsc []*csi.NodeServiceCapability
	for _, n := range nl {
		nsc = append(nsc, NewNodeServiceCapability(n))
	}
	d.nscap = nsc
}
