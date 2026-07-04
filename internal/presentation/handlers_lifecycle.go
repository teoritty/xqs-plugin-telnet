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

// HandleInitialize processes the initialize RPC.
func (h LifecycleHandlers) HandleInitialize(params json.RawMessage) (any, error) {
	var initParams struct {
		PluginID string `json:"pluginId"`
		DataDir  string `json:"dataDir"`
	}
	_ = json.Unmarshal(params, &initParams)
	if h.Log != nil {
		h.Log.Info("plugin initialized", map[string]string{
			"pluginId": initParams.PluginID,
		})
	}
	return map[string]bool{"ok": true}, nil
}

// HandleActivate processes the activate RPC.
func (h LifecycleHandlers) HandleActivate(params json.RawMessage) (any, error) {
	var activateParams struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(params, &activateParams)
	if h.Log != nil {
		h.Log.Info("plugin activated", map[string]string{
			"reason": activateParams.Reason,
		})
	}
	return map[string]bool{"ok": true}, nil
}

// HandleShutdown processes the shutdown RPC.
func (h LifecycleHandlers) HandleShutdown(_ json.RawMessage, manager *usecase.Manager) (any, error) {
	if manager != nil {
		manager.Disconnect()
	}
	return map[string]bool{"ok": true}, nil
}
