/*
 *
 * Copyright 2017 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package balancer defines APIs for load balancing in gRPC.
// All APIs in this package are experimental.
package balancer

import (
	"context"
	"encoding/json"
	"errors"
	"google.golang.org/grpc/balancer/apis"
	"net"
	"strings"

	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/internal"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

var (
	// m is a map from name to balancer builder.
	m = make(map[string]Builder)
)

// Register registers the balancer builder to the balancer map. b.Name
// (lowercased) will be used as the name registered with this builder.  If the
// Builder implements ConfigParser, ParseConfig will be called when new service
// configs are received by the resolver, and the result will be provided to the
// Balancer in UpdateClientConnState.
//
// NOTE: this function must only be called during initialization time (i.e. in
// an init() function), and is not thread-safe. If multiple Balancers are
// registered with the same name, the one registered last will take effect.
func Register(b Builder) {
	m[strings.ToLower(b.Name())] = b
}

// unregisterForTesting deletes the balancer with the given name from the
// balancer map.
//
// This function is not thread-safe.
func unregisterForTesting(name string) {
	delete(m, name)
}

func init() {
	internal.BalancerUnregister = unregisterForTesting
}

// Get returns the resolver builder registered with the given name.
// Note that the compare is done in a case-insensitive fashion.
// If no builder is register with the name, nil will be returned.
func Get(name string) Builder {
	if b, ok := m[strings.ToLower(name)]; ok {
		return b
	}
	return nil
}

// NewSubConnOptions contains options to create new SubConn.
type NewSubConnOptions struct {
	// CredsBundle is the credentials bundle that will be used in the created
	// SubConn. If it's nil, the original creds from grpc DialOptions will be
	// used.
	//
	// Deprecated: Use the Attributes field in resolver.Address to pass
	// arbitrary data to the credential handshaker.
	CredsBundle credentials.Bundle
	// HealthCheckEnabled indicates whether health check service should be
	// enabled on this SubConn
	HealthCheckEnabled bool
}

// State contains the balancer's state relevant to the gRPC ClientConn.
type State struct {
	// State contains the connectivity state of the balancer, which is used to
	// determine the state of the ClientConn.
	ConnectivityState connectivity.State
	// Picker is used to choose connections (SubConns) for RPCs.
	Picker Picker
}

// ClientConn represents a gRPC ClientConn.
//
// This interface is to be implemented by gRPC. Users should not need a
// brand new implementation of this interface. For the situations like
// testing, the new implementation should embed this interface. This allows
// gRPC to add new methods to this interface.
type ClientConn interface {
	// NewSubConn is called by balancer to create a new SubConn.
	// It doesn't block and wait for the connections to be established.
	// Behaviors of the SubConn can be controlled by options.
	NewSubConn([]resolver.Address, NewSubConnOptions) (apis.SubConn, error)
	// RemoveSubConn removes the SubConn from ClientConn.
	// The SubConn will be shutdown.
	RemoveSubConn(apis.SubConn)

	// UpdateState notifies gRPC that the balancer's internal state has
	// changed.
	//
	// gRPC will update the connectivity state of the ClientConn, and will call
	// Pick on the new Picker to pick new SubConns.
	UpdateState(State)

	// ResolveNow is called by balancer to notify gRPC to do a name resolving.
	ResolveNow(resolver.ResolveNowOptions)

	// Target returns the dial target for this ClientConn.
	//
	// Deprecated: Use the Target field in the BuildOptions instead.
	Target() string
}

// BuildOptions contains additional information for Build.
type BuildOptions struct {
	// DialCreds is the transport credential the Balancer implementation can
	// use to dial to a remote load balancer server. The Balancer implementations
	// can ignore this if it does not need to talk to another party securely.
	DialCreds credentials.TransportCredentials
	// CredsBundle is the credentials bundle that the Balancer can use.
	CredsBundle credentials.Bundle
	// Dialer is the custom dialer the Balancer implementation can use to dial
	// to a remote load balancer server. The Balancer implementations
	// can ignore this if it doesn't need to talk to remote balancer.
	Dialer func(context.Context, string) (net.Conn, error)
	// ChannelzParentID is the entity parent's channelz unique identification number.
	ChannelzParentID int64
	// CustomUserAgent is the custom user agent set on the parent ClientConn.
	// The balancer should set the same custom user agent if it creates a
	// ClientConn.
	CustomUserAgent string
	// Target contains the parsed address info of the dial target. It is the same resolver.Target as
	// passed to the resolver.
	// See the documentation for the resolver.Target type for details about what it contains.
	Target resolver.Target
}

// Builder creates a balancer.
type Builder interface {
	// Build creates a new balancer with the ClientConn.
	Build(cc ClientConn, opts BuildOptions) Balancer
	// Name returns the name of balancers built by this builder.
	// It will be used to pick balancers (for example in service config).
	Name() string
}

// ConfigParser parses load balancer configs.
type ConfigParser interface {
	// ParseConfig parses the JSON load balancer config provided into an
	// internal form or returns an error if the config is invalid.  For future
	// compatibility reasons, unknown fields in the config should be ignored.
	ParseConfig(LoadBalancingConfigJSON json.RawMessage) (serviceconfig.LoadBalancingConfig, error)
}

// PickInfo contains additional information for the Pick operation.
type PickInfo struct {
	// FullMethodName is the method name that NewClientStream() is called
	// with. The canonical format is /service/Method.
	FullMethodName string
	// Ctx is the RPC's context, and may contain relevant RPC-level information
	// like the outgoing header metadata.
	Ctx context.Context
}

// DoneInfo contains additional information for done.
type DoneInfo struct {
	// Err is the rpc error the RPC finished with. It could be nil.
	Err error
	// Trailer contains the metadata from the RPC's trailer, if present.
	Trailer metadata.MD
	// BytesSent indicates if any bytes have been sent to the server.
	BytesSent bool
	// BytesReceived indicates if any byte has been received from the server.
	BytesReceived bool
	// ServerLoad is the load received from server. It's usually sent as part of
	// trailing metadata.
	//
	// The only supported type now is *orca_v1.LoadReport.
	ServerLoad interface{}
}

var (
	// ErrNoSubConnAvailable indicates no SubConn is available for pick().
	// gRPC will block the RPC until a new picker is available via UpdateState().
	ErrNoSubConnAvailable = errors.New("no SubConn is available")
	// ErrTransientFailure indicates all SubConns are in TransientFailure.
	// WaitForReady RPCs will block, non-WaitForReady RPCs will fail.
	//
	// Deprecated: return an appropriate error based on the last resolution or
	// connection attempt instead.  The behavior is the same for any non-gRPC
	// status error.
	ErrTransientFailure = errors.New("all SubConns are in TransientFailure")
)

// PickResult contains information related to a connection chosen for an RPC.
type PickResult struct {
	// SubConn is the connection to use for this pick, if its state is Ready.
	// If the state is not Ready, gRPC will block the RPC until a new Picker is
	// provided by the balancer (using ClientConn.UpdateState).  The SubConn
	// must be one returned by ClientConn.NewSubConn.
	SubConn apis.SubConn

	// Done is called when the RPC is completed.  If the SubConn is not ready,
	// this will be called with a nil parameter.  If the SubConn is not a valid
	// type, Done may not be called.  May be nil if the balancer does not wish
	// to be notified when the RPC completes.
	Done func(DoneInfo)
}

// TransientFailureError returns e.  It exists for backward compatibility and
// will be deleted soon.
//
// Deprecated: no longer necessary, picker errors are treated this way by
// default.
func TransientFailureError(e error) error { return e }

// Picker is used by gRPC to pick a SubConn to send an RPC.
// Balancer is expected to generate a new picker from its snapshot every time its
// internal state has changed.
//
// The pickers used by gRPC can be updated by ClientConn.UpdateState().
type Picker interface {
	// Pick returns the connection to use for this RPC and related information.
	//
	// Pick should not block.  If the balancer needs to do I/O or any blocking
	// or time-consuming work to service this call, it should return
	// ErrNoSubConnAvailable, and the Pick call will be repeated by gRPC when
	// the Picker is updated (using ClientConn.UpdateState).
	//
	// If an error is returned:
	//
	// - If the error is ErrNoSubConnAvailable, gRPC will block until a new
	//   Picker is provided by the balancer (using ClientConn.UpdateState).
	//
	// - If the error is a status error (implemented by the grpc/status
	//   package), gRPC will terminate the RPC with the code and message
	//   provided.
	//
	// - For all other errors, wait for ready RPCs will wait, but non-wait for
	//   ready RPCs will be terminated with this error's Error() string and
	//   status code Unavailable.
	Pick(info PickInfo) (PickResult, error)
}

// Balancer takes input from gRPC, manages SubConns, and collects and aggregates
// the connectivity states.
//
// It also generates and updates the Picker used by gRPC to pick SubConns for RPCs.
//
// UpdateClientConnState, ResolverError, UpdateSubConnState, and Close are
// guaranteed to be called synchronously from the same goroutine.  There's no
// guarantee on picker.Pick, it may be called anytime.
type Balancer interface {
	// UpdateClientConnState is called by gRPC when the state of the ClientConn
	// changes.  If the error returned is ErrBadResolverState, the ClientConn
	// will begin calling ResolveNow on the active name resolver with
	// exponential backoff until a subsequent call to UpdateClientConnState
	// returns a nil error.  Any other errors are currently ignored.
	UpdateClientConnState(ClientConnState) error
	// ResolverError is called by gRPC when the name resolver reports an error.
	ResolverError(error)
	// UpdateSubConnState is called by gRPC when the state of a SubConn
	// changes.
	UpdateSubConnState(apis.SubConn, SubConnState)
	// Close closes the balancer. The balancer is not required to call
	// ClientConn.RemoveSubConn for its existing SubConns.
	Close()
}

// SubConnState describes the state of a SubConn.
type SubConnState struct {
	// ConnectivityState is the connectivity state of the SubConn.
	ConnectivityState connectivity.State
	// ConnectionError is set if the ConnectivityState is TransientFailure,
	// describing the reason the SubConn failed.  Otherwise, it is nil.
	ConnectionError error
}

// ClientConnState describes the state of a ClientConn relevant to the
// balancer.
type ClientConnState struct {
	ResolverState resolver.State
	// The parsed load balancing configuration returned by the builder's
	// ParseConfig method, if implemented.
	BalancerConfig serviceconfig.LoadBalancingConfig
}

// ErrBadResolverState may be returned by UpdateClientConnState to indicate a
// problem with the provided name resolver data.
var ErrBadResolverState = errors.New("bad resolver state")

// ConnectivityStateEvaluator takes the connectivity states of multiple SubConns
// and returns one aggregated connectivity state.
//
// It's not thread safe.
type ConnectivityStateEvaluator struct {
	numReady      uint64 // Number of addrConns in ready state.
	numConnecting uint64 // Number of addrConns in connecting state.
}

// RecordTransition records state change happening in subConn and based on that
// it evaluates what aggregated state should be.
//
//  - If at least one SubConn in Ready, the aggregated state is Ready;
//  - Else if at least one SubConn in Connecting, the aggregated state is Connecting;
//  - Else the aggregated state is TransientFailure.
//
// Idle and Shutdown are not considered.
func (cse *ConnectivityStateEvaluator) RecordTransition(oldState, newState connectivity.State) connectivity.State {
	// Update counters.
	for idx, state := range []connectivity.State{oldState, newState} {
		updateVal := 2*uint64(idx) - 1 // -1 for oldState and +1 for new.
		switch state {
		case connectivity.Ready:
			cse.numReady += updateVal
		case connectivity.Connecting:
			cse.numConnecting += updateVal
		}
	}

	// Evaluate.
	if cse.numReady > 0 {
		return connectivity.Ready
	}
	if cse.numConnecting > 0 {
		return connectivity.Connecting
	}
	return connectivity.TransientFailure
}
