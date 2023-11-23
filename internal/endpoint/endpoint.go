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

package endpoint

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"sync"
)

// Schemes supported by COSI.
const (
	SchemeUNIX = "unix"
	SchemeTCP  = "tcp"
)

var ErrEmptyAddress = errors.New("address is empty")

// Endpoint represents COSI Endpoint.
type Endpoint struct {
	once sync.Once

	address       *url.URL
	listener      net.Listener
	listenerError error
}

// Interface guards.
var _ io.Closer = (*Endpoint)(nil)

// New creates a new COSI Endpoint with the specified URL. The URL defines the communication
// protocol and address for the endpoint.
func New(url *url.URL) *Endpoint {
	return &Endpoint{
		address: url,
	}
}

// Listener will return listener (and error) after first configuring it.
// Listener is configured only once, on the first call, and the error is captured.
// Every subsequent call will return the same values.
func (e *Endpoint) Listener(ctx context.Context) (net.Listener, error) {
	e.once.Do(func() {
		listenConfig := net.ListenConfig{}

		if e.address == nil {
			e.listenerError = ErrEmptyAddress
			return
		}

		e.listener, e.listenerError = listenConfig.Listen(ctx, e.address.Scheme, e.address.Path)
		if e.listenerError != nil {
			return
		}
	})

	return e.listener, e.listenerError
}

// Close ensures that the UNIX socket is removed after the application is closed.
func (e *Endpoint) Close() error {
	if e.address != nil && e.address.Scheme == SchemeUNIX {
		return os.Remove(e.address.Path)
	}

	return nil
}
