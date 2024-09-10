// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelgrpc

// Role is a enum for setting the role of the instrumentation.
type Role int

const (
	RoleServer Role = iota
	RoleClient
)

// String will return the name for the role.
func (r Role) String() string {
	return [...]string{"server", "client"}[r]
}

func (r Role) isServer() bool {
	return r == RoleServer
}
