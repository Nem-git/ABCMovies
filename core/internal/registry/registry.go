package registry

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

const bufSize = 1024 * 1024

// Capability is a contract name and version a slot speaks (§3.3 of PLAN.md).
type Capability struct {
	Name    string
	Version uint32
}

// SlotInfo is what a slot declared at handshake: its capabilities plus any
// per-capability operating policy it declared alongside them.
type SlotInfo struct {
	Capabilities []Capability
	Policy       map[string]string
}

// Registry defines the slot registry operations used by the composition root.
type Registry interface {
	Admit(name string, server corev1.MetaServiceServer) ([]Capability, error)
	Close()
}

// InProcessRegistry handshakes declared slots and keeps a live table of what
// is admitted. It uses an in-process gRPC transport (bufconn).
type InProcessRegistry struct {
	slots map[string]*slotEntry
}

type slotEntry struct {
	capabilities []Capability
	policy       map[string]string
	server       *grpc.Server
	conn         *grpc.ClientConn
	listener     *bufconn.Listener
}

// NewInProcess returns an empty in-process registry.
func NewInProcess() *InProcessRegistry {
	return &InProcessRegistry{slots: map[string]*slotEntry{}}
}

// Admit handshakes an in-process slot over an in-memory transport: it serves
// the slot's Meta service on a buffer connection, asks CapabilityQuery, and
// validates the declaration. An invalid declaration is rejected (§3.3 of
// PLAN.md: nothing is assumed, everything is asked).
func (r *InProcessRegistry) Admit(name string, server corev1.MetaServiceServer) ([]Capability, error) {
	if _, exists := r.slots[name]; exists {
		return nil, fmt.Errorf("registry: slot %q already admitted", name)
	}
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	corev1.RegisterMetaServiceServer(srv, server)

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		_ = lis.Close()
		return nil, fmt.Errorf("registry: dial %q: %w", name, err)
	}

	go func() { _ = srv.Serve(lis) }()

	resp, err := corev1.NewMetaServiceClient(conn).CapabilityQuery(context.Background(), &corev1.CapabilityQueryRequest{})
	if err != nil {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
		return nil, fmt.Errorf("registry: handshake %q failed: %w", name, err)
	}

	caps := make([]Capability, 0, len(resp.GetCapabilities()))
	for _, c := range resp.GetCapabilities() {
		if c.GetName() == "" || c.GetVersion() == 0 {
			_ = conn.Close()
			srv.Stop()
			_ = lis.Close()
			return nil, fmt.Errorf("registry: %q declared invalid capability (name %q version %d)", name, c.GetName(), c.GetVersion())
		}
		caps = append(caps, Capability{Name: c.GetName(), Version: c.GetVersion()})
	}

	r.slots[name] = &slotEntry{
		capabilities: caps,
		policy:       resp.GetPolicy(),
		server:       srv,
		conn:         conn,
		listener:     lis,
	}
	return caps, nil
}

// Capabilities returns the admitted capabilities of a slot.
func (r *InProcessRegistry) Capabilities(name string) ([]Capability, bool) {
	entry, ok := r.slots[name]
	if !ok {
		return nil, false
	}
	return entry.capabilities, true
}

// Policy returns the operating policy a slot declared at handshake, e.g.
// {"browse.sync-cadence": "6h"}. Absent keys were simply not declared; the
// caller applies its own precedence around them. Unknown slot → nil, false.
func (r *InProcessRegistry) Policy(name string) (map[string]string, bool) {
	entry, ok := r.slots[name]
	if !ok {
		return nil, false
	}
	return entry.policy, true
}

// Snapshot returns every admitted slot with what it declared at handshake.
// The order of slots is not specified; callers that display them sort
// themselves. The returned maps are copies.
func (r *InProcessRegistry) Snapshot() map[string]SlotInfo {
	out := make(map[string]SlotInfo, len(r.slots))
	for name, entry := range r.slots {
		caps := make([]Capability, len(entry.capabilities))
		copy(caps, entry.capabilities)
		policy := make(map[string]string, len(entry.policy))
		for k, v := range entry.policy {
			policy[k] = v
		}
		out[name] = SlotInfo{Capabilities: caps, Policy: policy}
	}
	return out
}

// Close tears down every admitted slot's transport.
func (r *InProcessRegistry) Close() {
	for _, entry := range r.slots {
		_ = entry.conn.Close()
		entry.server.Stop()
		_ = entry.listener.Close()
	}
}
