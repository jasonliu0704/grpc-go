package customRoundrobin

import (
	"google.golang.org/grpc/balancer/apis"
	"strings"
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/internal/grpcrand"
)

const Name = "customRoundrobin"
const OverWriteKeyName = "lb-addr"

var logger = grpclog.Component("customRoundrobin")

// newBuilder creates a new roundrobin balancer builder.
func newBuilder() balancer.Builder {
	return base.NewBalancerBuilder(Name, &rrPickerBuilder{}, base.Config{HealthCheck: true})
}

func init() {
	balancer.Register(newBuilder())
}

type rrPickerBuilder struct{}

func (*rrPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	logger.Infof("customRoundrobin: newPicker called with info: %v", info)
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}
	var scs []apis.SubConn
	for sc := range info.ReadySCs {
		scs = append(scs, sc)
	}
	return &rrPicker{
		subConns: scs,
		// Start at a random index, as the same RR balancer rebuilds a new
		// picker when SubConn states change, and we don't want to apply excess
		// load to the first server in the list.
		next: grpcrand.Intn(len(scs)),
	}
}

type rrPicker struct {
	// subConns is the snapshot of the customRoundrobin balancer when this picker was
	// created. The slice is immutable. Each Get() will do a round robin
	// selection from it and return the selected SubConn.
	subConns []apis.SubConn

	mu   sync.Mutex
	next int
}

/*
Pick is the core logic of custom rooundrobin
For stateful load balancing, we look for the "lb-addr" from the context,
if the addr is present, we need to route request to the addr as overwritten,
if not, we switch to the regular roundrobin
 */
func (p *rrPicker) Pick(pi balancer.PickInfo) (balancer.PickResult, error) {
	p.mu.Lock()

	var chosenSc apis.SubConn

	// subConn pick on user request
	if overwriteAddr, ok := pi.Ctx.Value(OverWriteKeyName).(string); ok {
		for _, sc := range p.subConns {
			curAddr := sc.GetAddrConnection().GetCurAddr()	//reflect.ValueOf(sc).Elem().FieldByName("ac").Interface().(*addrConn)
			if strings.Compare(curAddr.Addr, overwriteAddr) == 0 {
				// add match, route to the subconnection
				chosenSc = sc
			}
		}
	}else{
		// subConn pick on lb
		chosenSc = p.subConns[p.next]
		p.next = (p.next + 1) % len(p.subConns)
	}

	p.mu.Unlock()
	return balancer.PickResult{SubConn: chosenSc}, nil
}