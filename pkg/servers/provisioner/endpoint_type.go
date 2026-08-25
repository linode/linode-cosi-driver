// Copyright 2023-2025 Akamai Technologies, Inc.
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
	"fmt"
	"strings"

	"github.com/linode/linodego/v2"
)

func parseEndpointType(params map[string]string) (linodego.ObjectStorageEndpointType, error) {
	endpointType := linodego.ObjectStorageEndpointType(params[ParamEndpointType])
	if endpointType == "" {
		return "", nil
	}

	switch endpointType {
	case linodego.ObjectStorageEndpointE0,
		linodego.ObjectStorageEndpointE1,
		linodego.ObjectStorageEndpointE2,
		linodego.ObjectStorageEndpointE3:
		return endpointType, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnknownEndpointType, endpointType)
	}
}

func parseEndpointTypePreference(params map[string]string) ([]linodego.ObjectStorageEndpointType, error) {
	if endpointType, err := parseEndpointType(params); err != nil || endpointType != "" {
		return []linodego.ObjectStorageEndpointType{endpointType}, err
	}

	value := strings.TrimSpace(params[ParamEndpointTypePreference])
	if value == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	preferences := make([]linodego.ObjectStorageEndpointType, 0, len(parts))
	for _, part := range parts {
		endpointType := linodego.ObjectStorageEndpointType(strings.TrimSpace(part))
		if endpointType == "" {
			continue
		}

		switch endpointType {
		case linodego.ObjectStorageEndpointE0,
			linodego.ObjectStorageEndpointE1,
			linodego.ObjectStorageEndpointE2,
			linodego.ObjectStorageEndpointE3:
			preferences = append(preferences, endpointType)
		default:
			return nil, fmt.Errorf("%w: %s", ErrUnknownEndpointType, endpointType)
		}
	}

	if len(preferences) == 0 {
		return nil, nil
	}

	return preferences, nil
}
