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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	"github.com/sys-liqian/csi-driver-webdav/pkg/webdav/mount"
)

type ControllerServer struct {
	*Driver
	mounter mount.Interface
	csi.UnimplementedControllerServer
}

func NewControllerServer(d *Driver, mounter mount.Interface) *ControllerServer {
	return &ControllerServer{
		Driver:  d,
		mounter: mounter,
	}
}

// CreateVolume implements csi.ControllerServer.
func (c *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}
	if err := isValidVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	mountPermissions := c.Driver.mountPermissions
	parameters := req.GetParameters()
	if parameters == nil {
		parameters = make(map[string]string)
	}
	for k, v := range parameters {
		switch strings.ToLower(k) {
		case webdavSharePath, pvcNameKey, pvcNamespaceKey, pvNameKey:
		case mountPermissionsField:
			if v != "" {
				var err error
				if mountPermissions, err = strconv.ParseUint(v, 8, 32); err != nil {
					return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("invalid mountPermissions %s in storage class", v))
				}
			}
		default:
			return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("invalid parameter %q in storage class", k))
		}
	}

	targetPath := c.workingMountDir
	sourcePath := req.Parameters[webdavSharePath]
	notMnt, err := c.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(targetPath, 0750); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			notMnt = true
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if !notMnt {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("target path %s is alredy mounted", targetPath))
	}

	stdin := []string{req.GetSecrets()[secretUsernameKey], req.GetSecrets()[secretPasswordKey]}
	if err := c.mounter.MountSensitiveWithStdin(sourcePath, targetPath, fstype, nil, nil, stdin); err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("mount failed: %v", err.Error()))
	}

	internalVolumePath := filepath.Join(targetPath, req.Name)
	if err = os.Mkdir(internalVolumePath, 0777); err != nil && !os.IsExist(err) {
		return nil, status.Errorf(codes.Internal, "failed to make subdirectory: %v", err.Error())
	}

	defer func() {
		if err = c.mounter.Unmount(targetPath); err != nil {
			klog.Warningf("failed to unmount targetpath %s: %v", targetPath, err.Error())
		}
	}()

	if mountPermissions > 0 {
		// Reset directory permissions because of umask problems
		if err = os.Chmod(internalVolumePath, os.FileMode(mountPermissions)); err != nil {
			klog.Warningf("failed to chmod subdirectory: %v", err.Error())
		}
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      MakeVolumeId(sourcePath, req.Name),
			CapacityBytes: 0, // by setting it to zero, Provisioner will use PVC requested size as PV size
			VolumeContext: nil,
			ContentSource: req.GetVolumeContentSource(),
		},
	}, nil

}

// DeleteVolume implements csi.ControllerServer.
func (c *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	sourcePath, subDir, err := ParseVolumeId(volumeID)
	if err != nil {
		// An invalid ID should be treated as doesn't exist
		klog.Warningf("failed to parse volume for volume id %v deletion: %v", volumeID, err)
		return &csi.DeleteVolumeResponse{}, nil
	}

	stdin := []string{req.GetSecrets()[secretUsernameKey], req.GetSecrets()[secretPasswordKey]}
	targetPath := c.workingMountDir
	if err := c.mounter.MountSensitiveWithStdin(sourcePath, targetPath, fstype, nil, nil, stdin); err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("mount failed: %v", err.Error()))
	}

	defer func() {
		if err = c.mounter.Unmount(targetPath); err != nil {
			klog.Warningf("failed to unmount targetpath %s: %v", targetPath, err.Error())
		}
	}()

	internalVolumePath := filepath.Join(targetPath, subDir)
	klog.V(2).Infof("Removing subdirectory at %v", internalVolumePath)
	if err = os.RemoveAll(internalVolumePath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete subdirectory: %v", err.Error())
	}

	return &csi.DeleteVolumeResponse{}, nil
}

// ValidateVolumeCapabilities implements csi.ControllerServer.
func (c *ControllerServer) ValidateVolumeCapabilities(_ context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if err := isValidVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
		},
		Message: "",
	}, nil
}

// ControllerGetCapabilities implements csi.ControllerServer.
func (c *ControllerServer) ControllerGetCapabilities(context.Context, *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: c.Driver.cscap,
	}, nil
}

// ControllerExpandVolume implements csi.ControllerServer.
func (*ControllerServer) ControllerExpandVolume(context.Context, *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerGetVolume implements csi.ControllerServer.
func (*ControllerServer) ControllerGetVolume(context.Context, *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerModifyVolume implements csi.ControllerServer.
func (*ControllerServer) ControllerModifyVolume(context.Context, *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerPublishVolume implements csi.ControllerServer.
func (*ControllerServer) ControllerPublishVolume(context.Context, *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerUnpublishVolume implements csi.ControllerServer.
func (*ControllerServer) ControllerUnpublishVolume(context.Context, *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// CreateSnapshot implements csi.ControllerServer.
func (*ControllerServer) CreateSnapshot(context.Context, *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// DeleteSnapshot implements csi.ControllerServer.
func (*ControllerServer) DeleteSnapshot(context.Context, *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// GetCapacity implements csi.ControllerServer.
func (*ControllerServer) GetCapacity(context.Context, *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ListSnapshots implements csi.ControllerServer.
func (*ControllerServer) ListSnapshots(context.Context, *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ListVolumes implements csi.ControllerServer.
func (*ControllerServer) ListVolumes(context.Context, *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// isValidVolumeCapabilities validates the given VolumeCapability array is valid
func isValidVolumeCapabilities(volCaps []*csi.VolumeCapability) error {
	if len(volCaps) == 0 {
		return fmt.Errorf("volume capabilities missing in request")
	}
	for _, c := range volCaps {
		if c.GetBlock() != nil {
			return fmt.Errorf("block volume capability not supported")
		}
	}
	return nil
}
