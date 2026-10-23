// Package di wires the preferences module: it builds the adapters and the use cases the composition
// root (internal/app) needs.
package di

import (
	"database/sql"

	"iworkflow/internal/modules/preferences/application"
	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/outbound/persistence"
)

// Deps are the shared resources the module is built from.
type Deps struct {
	DB *sql.DB
}

// Module exposes the preferences use cases. Handlers and middlewares are mounted by the composition
// root, which passes them these use cases.
type Module struct {
	GetLanguage            *application.GetLanguage
	SaveLanguage           *application.SaveLanguage
	GetNetworkAccess       *application.GetNetworkAccess
	SaveNetworkAccess      *application.SaveNetworkAccess
	GetListenPort          *application.GetListenPort
	SaveListenPort         *application.SaveListenPort
	GetRequireAccessToken  *application.GetRequireAccessToken
	SaveRequireAccessToken *application.SaveRequireAccessToken
	GetAccessToken         *application.GetAccessToken
	RegenerateAccessToken  *application.RegenerateAccessToken
}

// NewModule builds the repository once and hands it to every use case.
func NewModule(deps Deps) *Module {
	repository := persistence.NewPreferencesRepository(deps.DB)
	return &Module{
		GetLanguage:            application.NewGetLanguage(repository),
		SaveLanguage:           application.NewSaveLanguage(repository),
		GetNetworkAccess:       application.NewGetNetworkAccess(repository),
		SaveNetworkAccess:      application.NewSaveNetworkAccess(repository),
		GetListenPort:          application.NewGetListenPort(repository),
		SaveListenPort:         application.NewSaveListenPort(repository),
		GetRequireAccessToken:  application.NewGetRequireAccessToken(repository),
		SaveRequireAccessToken: application.NewSaveRequireAccessToken(repository),
		GetAccessToken:         application.NewGetAccessToken(repository, models.GenerateAccessToken),
		RegenerateAccessToken:  application.NewRegenerateAccessToken(repository, models.GenerateAccessToken),
	}
}
