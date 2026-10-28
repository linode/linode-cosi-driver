// Copyright 2023 Akamai Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provisioner

import (
	"strings"
	"testing"

	"github.com/linode/linodego/v2"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	cosi "sigs.k8s.io/container-object-storage-interface/proto"

	"github.com/linode/linode-cosi-driver/testing/mock"
)

func TestDriverGenerateBucketId(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"bc-12345678-abcd-1234-abcd-123456789abc", strings.Repeat("a", 100)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server := &Server{}
			req := &cosi.DriverGenerateBucketIdRequest{
				Name:       name,
				Parameters: map[string]string{ParamRegion: "us-ord", ParamCleanup: "force"},
			}
			first, err := server.DriverGenerateBucketId(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			second, err := server.DriverGenerateBucketId(t.Context(), req)
			if err != nil || first.GetBucketId() != second.GetBucketId() {
				t.Fatalf("identity changed on retry: %v", err)
			}
			region, label, cleanup, err := parseBucketID(first.GetBucketId())
			if err != nil || region != "us-ord" || len(label) > maxBucketLabelLength || !cleanup {
				t.Fatalf("invalid generated identity: %q (%v)", first.GetBucketId(), err)
			}
		})
	}
}

func TestDriverGetBucket(t *testing.T) {
	t.Parallel()
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing", true: "missing"}[missing], func(t *testing.T) {
			t.Parallel()
			client := mock.NewMockLinodeClient(gomock.NewController(t))
			var bucket *linodego.ObjectStorageBucket
			var backendErr error
			if missing {
				backendErr = ErrNotFound
			} else {
				bucket = &linodego.ObjectStorageBucket{Region: "us-ord", Label: "example", S3Endpoint: "us-ord.linodeobjects.com"}
			}
			client.EXPECT().GetObjectStorageBucket(gomock.Any(), "us-ord", "example").Return(bucket, backendErr)
			server := &Server{client: client}
			response, err := server.DriverGetBucket(t.Context(), &cosi.DriverGetBucketRequest{BucketId: "us-ord/example"})
			if missing {
				if status.Code(err) != codes.NotFound {
					t.Fatalf("expected NotFound, got %v", err)
				}
				return
			}
			if err != nil || response.GetProtocols().GetS3().GetBucketId() != "example" || response.GetProtocols().GetS3().GetEndpoint() == "" {
				t.Fatalf("invalid static bucket response: %v", err)
			}
		})
	}
}

func TestDriverGrantBucketAccessUnsupportedRequests(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		mode  cosi.AccessMode_Mode
		count int
		want  codes.Code
	}{
		{name: "write-only", mode: cosi.AccessMode_WRITE_ONLY, count: 1, want: codes.InvalidArgument},
		{name: "no requested access mode", mode: cosi.AccessMode_ANY, count: 1, want: codes.InvalidArgument},
		{name: "multiple buckets", mode: cosi.AccessMode_READ_WRITE, count: 2, want: codes.OutOfRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := &cosi.DriverGrantBucketAccessRequest{
				AccountName:        "example-access",
				Protocol:           &cosi.ObjectProtocol{Type: cosi.ObjectProtocol_S3},
				AuthenticationType: &cosi.AuthenticationType{Type: cosi.AuthenticationType_KEY},
			}
			for range tc.count {
				req.Buckets = append(req.Buckets, &cosi.DriverGrantBucketAccessRequest_AccessedBucket{
					BucketId: "us-ord/example", ObjectDataAccessMode: &cosi.AccessMode{Mode: tc.mode},
				})
			}
			server := &Server{}
			if _, err := server.DriverGrantBucketAccess(t.Context(), req); status.Code(err) != tc.want {
				t.Fatalf("expected %s, got %v", tc.want, err)
			}
		})
	}
}

func TestDriverRevokeBucketAccessWithoutBucketResource(t *testing.T) {
	t.Parallel()
	client := mock.NewMockLinodeClient(gomock.NewController(t))
	client.EXPECT().DeleteObjectStorageKey(gomock.Any(), 42).Return(ErrNotFound)
	server := &Server{client: client}
	_, err := server.DriverRevokeBucketAccess(t.Context(), &cosi.DriverRevokeBucketAccessRequest{
		AccountId: "42", Protocol: &cosi.ObjectProtocol{Type: cosi.ObjectProtocol_S3},
	})
	if err != nil {
		t.Fatalf("already revoked access should succeed without a bucket lookup: %v", err)
	}
}
