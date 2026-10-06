package registry

import (
	"context"
	"fmt"
	"net"
	"sync"

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
	mu    sync.Mutex
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
// validates the declaration, then records what the slot declared. An invalid
// declaration is rejected (§3.3 of PLAN.md: nothing is assumed, everything is
// asked). Publication is Admit's only side effect — Describe skips it.
func (r *InProcessRegistry) Admit(name string, server corev1.MetaServiceServer) ([]Capability, error) {
	info, err := r.handshake(name, server, true)
	if err != nil {
		return nil, err
	}
	return info.Capabilities, nil
}

// Describe runs the same serve-handshake-validate against a slot as Admit,
// but publishes nothing and tears the probe transport down: factories use it
// to resolve what a slot declared (its capabilities and policy, e.g. the
// sync cadence) before the composition root publishes anything.
func (r *InProcessRegistry) Describe(name string, server corev1.MetaServiceServer) (SlotInfo, error) {
	return r.handshake(name, server, false)
}

// handshake serves the slot's Meta service on a buffer connection, asks
// CapabilityQuery and validates the declaration. When publish is true the
// slot is recorded in the registry; when false the probe transport is torn
// down and only the declaration is returned.
func (r *InProcessRegistry) handshake(name string, server corev1.MetaServiceServer, publish bool) (SlotInfo, error) {
	if publish {
		r.mu.Lock()
		if _, exists := r.slots[name]; exists {
			r.mu.Unlock()
			return SlotInfo{}, fmt.Errorf("registry: slot %q already admitted", name)
		}
		r.mu.Unlock()
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
		return SlotInfo{}, fmt.Errorf("registry: dial %q: %w", name, err)
	}

	go func() { _ = srv.Serve(lis) }()

	resp, err := corev1.NewMetaServiceClient(conn).CapabilityQuery(context.Background(), &corev1.CapabilityQueryRequest{})
	if err != nil {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
		return SlotInfo{}, fmt.Errorf("registry: handshake %q failed: %w", name, err)
	}

	caps := make([]Capability, 0, len(resp.GetCapabilities()))
	for _, c := range resp.GetCapabilities() {
		if c.GetName() == "" || c.GetVersion() == 0 {
			_ = conn.Close()
			srv.Stop()
			_ = lis.Close()
			return SlotInfo{}, fmt.Errorf("registry: %q declared invalid capability (name %q version %d)", name, c.GetName(), c.GetVersion())
		}
		caps = append(caps, Capability{Name: c.GetName(), Version: c.GetVersion()})
	}

	info := SlotInfo{Capabilities: caps, Policy: resp.GetPolicy()}
	if !publish {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
		return info, nil
	}
	r.mu.Lock()
	r.slots[name] = &slotEntry{
		capabilities: caps,
		policy:       info.Policy,
		server:       srv,
		conn:         conn,
		listener:     lis,
	}
	r.mu.Unlock()
	return info, nil
}

// Forget dismisses a slot and tears its transport down: the slot may not
// serve or be queried again. It is the retirement half of Admit — used when
// a slot that no account needs any longer (e.g. the last account of a
// user-provided server is unlinked) goes away; an entry that was never
// admitted is a no-op, not an error.
func (r *InProcessRegistry) Forget(name string) {
	r.mu.Lock()
	entry, ok := r.slots[name]
	if ok {
		delete(r.slots, name)
	}
	r.mu.Unlock()
	if !ok {
		return
	}
	_ = entry.conn.Close()
	entry.server.Stop()
	_ = entry.listener.Close()
}

// Capabilities returns the admitted capabilities of a slot.
func (r *InProcessRegistry) Capabilities(name string) ([]Capability, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
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
	r.mu.Lock()
	defer r.mu.Unlock()
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
	r.mu.Lock()
	defer r.mu.Unlock()
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
	r.mu.Lock()
	slots := make([]*slotEntry, 0, len(r.slots))
	for _, entry := range r.slots {
		slots = append(slots, entry)
	}
	r.slots = map[string]*slotEntry{}
	r.mu.Unlock()
	for _, entry := range slots {
		_ = entry.conn.Close()
		entry.server.Stop()
		_ = entry.listener.Close()
	}
}
