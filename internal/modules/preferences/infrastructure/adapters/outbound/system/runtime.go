package system

import (
	"iworkflow/internal/core/listen"
	"iworkflow/internal/modules/preferences/domain/models"
)

// ServerRuntime describes the running server from the listen information chosen at start-up.
type ServerRuntime struct {
	info listen.Info
}

// NewServerRuntime wraps the listen information of the running server.
func NewServerRuntime(info listen.Info) *ServerRuntime {
	return &ServerRuntime{info: info}
}

// Status maps the listen information to the domain description.
func (r *ServerRuntime) Status() models.ServerStatus {
	return models.ServerStatus{
		Port:          r.info.Addr.Port(),
		Exposed:       r.info.IsExposed(),
		LockedByEnv:   r.info.LockedByEnv,
		TokenRequired: r.info.TokenRequired,
	}
}
