package apis

import (
	"google.golang.org/grpc/resolver"
)

// SubConn represents a gRPC sub connection.
// Each sub connection contains a list of addresses. gRPC will
// try to connect to them (in sequence), and stop trying the
// remainder once one connection is successful.
//
// The reconnect backoff will be applied on the list, not a single address.
// For example, try_on_all_addresses -> backoff -> try_on_all_addresses.
//
// All SubConns start in IDLE, and will not try to connect. To trigger
// the connecting, Balancers must call Connect.
// When the connection encounters an error, it will reconnect immediately.
// When the connection becomes IDLE, it will not reconnect unless Connect is
// called.
//
// This interface is to be implemented by gRPC. Users should not need a
// brand new implementation of this interface. For the situations like
// testing, the new implementation should embed this interface. This allows
// gRPC to add new methods to this interface.
type SubConn interface {
	// UpdateAddresses updates the addresses used in this SubConn.
	// gRPC checks if currently-connected address is still in the new list.
	// If it's in the list, the connection will be kept.
	// If it's not in the list, the connection will gracefully closed, and
	// a new connection will be created.
	//
	// This will trigger a state transition for the SubConn.
	UpdateAddresses([]resolver.Address)
	// Connect starts the connecting for this SubConn.
	Connect()
	// gets the addressConnection
	GetAddrConnection() resolver.Address
}
