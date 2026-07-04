package rpc

import "context"

// Caller abstracts outbound RPC to the xQuakShell core.
type Caller interface {
	CallCoreContext(ctx context.Context, method string, params any) ([]byte, error)
}

// HostCaller adapts Host to Caller.
type HostCaller struct {
	Host *Host
}

// CallCoreContext implements Caller.
func (c HostCaller) CallCoreContext(ctx context.Context, method string, params any) ([]byte, error) {
	if c.Host == nil {
		return nil, context.Canceled
	}
	return c.Host.CallCoreContext(ctx, method, params)
}
