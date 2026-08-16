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

// Registry is the wiring cabinet (§4 of PLAN.md): it handshakes declared
// slots and keeps a live table of what is admitted.
type Registry struct {
	slots map[string]*slotEntry
}

type slotEntry struct {
	capabilities []Capability
	server       *grpc.Server
	conn         *grpc.ClientConn
	listener     *bufconn.Listener
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{slots: map[string]*slotEntry{}}
}

// Admit handshakes an in-process slot over an in-memory transport: it serves
// the slot's Meta service on a buffer connection, asks CapabilityQuery, and
// validates the declaration. An invalid declaration is rejected (§3.3 of
// PLAN.md: nothing is assumed, everything is asked).
func (r *Registry) Admit(name string, server corev1.MetaServiceServer) ([]Capability, error) {
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

	r.slots[name] = &slotEntry{capabilities: caps, server: srv, conn: conn, listener: lis}
	return caps, nil
}

// Capabilities returns the admitted capabilities of a slot.
func (r *Registry) Capabilities(name string) ([]Capability, bool) {
	entry, ok := r.slots[name]
	if !ok {
		return nil, false
	}
	return entry.capabilities, true
}

// Close tears down every admitted slot's transport.
func (r *Registry) Close() {
	for _, entry := range r.slots {
		_ = entry.conn.Close()
		entry.server.Stop()
		_ = entry.listener.Close()
	}
}
