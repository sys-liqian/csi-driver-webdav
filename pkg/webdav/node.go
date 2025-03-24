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
	"context"
	"os"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	"github.com/sys-liqian/csi-driver-webdav/pkg/webdav/mount"
)

type NodeServer struct {
	Driver  *Driver
	mounter mount.Interface
	csi.UnimplementedNodeServer
}

func NewNodeServer(d *Driver, mounter mount.Interface) *NodeServer {
	return &NodeServer{
		Driver:  d,
		mounter: mounter,
	}
}

// NodePublishVolume implements csi.NodeServer.
func (n *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	mountOptions := volCap.GetMount().GetMountFlags()
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	}

	address, subDir, err := ParseVolumeId(volumeID)
	if err != nil {
		// An invalid ID should be treated as doesn't exist
		klog.Warningf("failed to parse volume for volume id %v deletion: %v", volumeID, err)
		return &csi.NodePublishVolumeResponse{}, nil
	}

	notMnt, err := n.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(targetPath, os.FileMode(n.Driver.mountPermissions)); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			notMnt = true
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if !notMnt {
		return &csi.NodePublishVolumeResponse{}, nil
	}

	sourcePath := strings.Join([]string{address, subDir}, "/")
	stdin := []string{req.GetSecrets()[secretUsernameKey], req.GetSecrets()[secretPasswordKey]}
	klog.V(2).Infof("NodePublishVolume: volumeID(%v) source(%s) targetPath(%s) mountflags(%v)", volumeID, sourcePath, targetPath, mountOptions)
	err = n.mounter.MountSensitiveWithStdin(sourcePath, targetPath, fstype, mountOptions, nil, stdin)
	if err != nil {
		if os.IsPermission(err) {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		if strings.Contains(err.Error(), "invalid argument") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume implements csi.NodeServer.
func (n *NodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	notMnt, err := n.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Error(codes.NotFound, "Targetpath not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if notMnt {
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	klog.V(2).Infof("NodeUnpublishVolume: unmounting volume %s on %s", volumeID, targetPath)
	err = n.mounter.Unmount(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount target %q: %v", targetPath, err)
	}

	klog.V(2).Infof("NodeUnpublishVolume: unmount volume %s on %s successfully", volumeID, targetPath)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetInfo implements csi.NodeServer.
func (n *NodeServer) NodeGetInfo(context.Context, *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{NodeId: n.Driver.nodeID}, nil
}

// NodeGetCapabilities implements csi.NodeServer.
func (n *NodeServer) NodeGetCapabilities(context.Context, *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: n.Driver.nscap,
	}, nil
}

// NodeExpandVolume implements csi.NodeServer.
func (*NodeServer) NodeExpandVolume(context.Context, *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeGetVolumeStats implements csi.NodeServer.
func (*NodeServer) NodeGetVolumeStats(context.Context, *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeStageVolume implements csi.NodeServer.
func (*NodeServer) NodeStageVolume(context.Context, *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeUnstageVolume implements csi.NodeServer.
func (*NodeServer) NodeUnstageVolume(context.Context, *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
