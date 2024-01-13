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

import "github.com/linode/linode-cosi-driver/pkg/observability/metrics"

// registerMetrics is the common place of registering new metrics to the server.
// When creating new metrics from the meter1, we call something like:
//
//	counter, err := meter.Float64Counter("example")
//
// As we expect the metrics to be registered, it is important to return and handle the error.
func (s *Server) registerMetrics() error {
	_ = metrics.Meter()

	// TODO: any new metrics should be placed here.

	return nil
}
