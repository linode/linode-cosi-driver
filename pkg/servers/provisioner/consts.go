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

const (
	ParamRegion = "cosi.linode.com/v1/region"
	ParamACL    = "cosi.linode.com/v1/acl"
	ParamCORS   = "cosi.linode.com/v1/cors"

	ParamCORSValueEnabled  ParamCORSValue = "enabled"
	ParamCORSValueDisabled ParamCORSValue = "disabled"
)

type ParamCORSValue string

func (v ParamCORSValue) Bool() bool {
	return v == ParamCORSValueEnabled
}

const (
	S3                      = "s3"
	S3Region                = "region"
	S3Endpoint              = "endpoint"
	S3SecretAccessKeyID     = "accessKeyID"
	S3SecretAccessSecretKey = "accessSecretKey"
)
