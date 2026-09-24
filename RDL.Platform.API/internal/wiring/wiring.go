// Package wiring arma los handlers HTTP sobre los adapters de Postgres. Lo usan cmd/api y la suite de aislamiento
// (tests/isolation), para que la suite pruebe exactamente los mismos endpoints que se despliegan.
package wiring

import (
	"log/slog"

	httpadapter "rdl/platform-api/internal/adapters/http"
	"rdl/platform-api/internal/adapters/postgres"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/platform/health"
	"rdl/platform-api/pkg/tenancy"
)

// Deps completa httpadapter.Deps con todos los casos de uso. verifier y health vienen de afuera porque la suite
// de aislamiento usa un verificador de prueba (no puede firmar tokens de Supabase).
func Deps(log *slog.Logger, checks *health.Handler, verifier tenancy.TokenVerifier,
	memberships *tenancy.CachedResolver, txm *postgres.TxManager) httpadapter.Deps {
	return httpadapter.Deps{
		Log:         log,
		Health:      checks,
		Verifier:    verifier,
		Memberships: memberships,
		Me: &httpadapter.MeHandlers{
			GetMe:                    app.NewGetMe(txm),
			ListMyMemberships:        app.NewListMyMemberships(txm),
			SelectActiveOrganization: app.NewSelectActiveOrganization(txm),
		},
		Organizations: &httpadapter.OrganizationHandlers{
			Create: app.NewCreateOrganization(txm),
			Get:    app.NewGetCurrentOrganization(txm),
			Update: app.NewUpdateCurrentOrganization(txm),
		},
		Members: &httpadapter.MemberHandlers{
			List:   app.NewListMembers(txm),
			Update: app.NewUpdateMember(txm, memberships),
		},
		Invitations: &httpadapter.InvitationHandlers{
			Create: app.NewCreateInvitation(txm),
			List:   app.NewListInvitations(txm),
			Revoke: app.NewRevokeInvitation(txm),
			Accept: app.NewAcceptInvitation(txm),
		},
		Branches: &httpadapter.BranchHandlers{
			Create: app.NewCreateBranch(txm),
			List:   app.NewListBranches(txm),
			Get:    app.NewGetBranch(txm),
			Update: app.NewUpdateBranch(txm),
		},
	}
}
