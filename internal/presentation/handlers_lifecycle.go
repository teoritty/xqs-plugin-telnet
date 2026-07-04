package presentation

import (
	"encoding/json"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
	"github.com/teoritty/xqs-plugin-telnet/internal/usecase"
)

// LifecycleHandlers wires plugin lifecycle RPC handlers.
type LifecycleHandlers struct {
	Log domain.LoggerPort
}

type lifecycleResult struct {
	ok bool
	log func()
}

func (r lifecycleResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]bool{"ok": r.ok})
}

func (r lifecycleResult) AfterResponse() {
	if r.log != nil {
		r.log()
	}
}

// HandleInitialize processes the initialize RPC.
func (h LifecycleHandlers) HandleInitialize(params json.RawMessage) (any, error) {
	var initParams struct {
		PluginID string `json:"pluginId"`
		DataDir  string `json:"dataDir"`
	}
	_ = json.Unmarshal(params, &initParams)

	var after func()
	if h.Log != nil {
		log := h.Log
		pluginID := initParams.PluginID
		after = func() {
			log.Info("plugin initialized", map[string]string{
				"pluginId": pluginID,
			})
		}
	}
	return lifecycleResult{ok: true, log: after}, nil
}

// HandleActivate processes the activate RPC.
func (h LifecycleHandlers) HandleActivate(params json.RawMessage) (any, error) {
	var activateParams struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(params, &activateParams)

	var after func()
	if h.Log != nil {
		log := h.Log
		reason := activateParams.Reason
		after = func() {
			log.Info("plugin activated", map[string]string{
				"reason": reason,
			})
		}
	}
	return lifecycleResult{ok: true, log: after}, nil
}

// HandleShutdown processes the shutdown RPC.
func (h LifecycleHandlers) HandleShutdown(_ json.RawMessage, manager *usecase.Manager) (any, error) {
	if manager != nil {
		go manager.Disconnect()
	}
	return map[string]bool{"ok": true}, nil
}
