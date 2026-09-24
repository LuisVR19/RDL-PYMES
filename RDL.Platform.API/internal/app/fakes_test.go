package app

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/branch"
	"rdl/platform-api/internal/domain/invitation"
	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/internal/domain/organization"
	"rdl/platform-api/internal/domain/user"
	"rdl/platform-api/pkg/tenancy"
)

// store es una base en memoria con semántica transaccional mínima: si fn falla, se descartan sus cambios.
type store struct {
	users        map[string]user.User // por sujeto
	memberships  map[uuid.UUID][]membership.Summary
	orgs         map[uuid.UUID]organization.Organization
	idem         map[string]IdempotencyRecord // clave: org + "|" + key
	tenantTxs    []tenancy.Context
	members      map[uuid.UUID]membership.Member // por userID (una sola organización en estos tests)
	invitations  map[uuid.UUID]invitation.Invitation
	tokenHashes  map[string]uuid.UUID // hash → invitación
	branches     map[uuid.UUID]branch.Branch
	audit        []AuditEvent
	txUsers      []uuid.UUID // sesión de usuario de cada transacción abierta
	raceOnCreate bool        // simula que otra transacción creó el usuario entre la búsqueda y el insert
}

func newStore() *store {
	return &store{
		users: map[string]user.User{}, memberships: map[uuid.UUID][]membership.Summary{},
		orgs: map[uuid.UUID]organization.Organization{}, idem: map[string]IdempotencyRecord{},
		members:     map[uuid.UUID]membership.Member{},
		invitations: map[uuid.UUID]invitation.Invitation{}, tokenHashes: map[string]uuid.UUID{},
		branches: map[uuid.UUID]branch.Branch{},
	}
}

func (s *store) WithinUserTx(ctx context.Context, userID uuid.UUID, fn func(context.Context, Tx) error) error {
	s.txUsers = append(s.txUsers, userID)
	return s.run(ctx, fn)
}

func (s *store) WithinTenantTx(ctx context.Context, t tenancy.Context, fn func(context.Context, Tx) error) error {
	s.tenantTxs = append(s.tenantTxs, t)
	return s.run(ctx, fn)
}

func (s *store) run(ctx context.Context, fn func(context.Context, Tx) error) error {
	usersBefore, orgsBefore, idemBefore := maps.Clone(s.users), maps.Clone(s.orgs), maps.Clone(s.idem)
	membershipsBefore, membersBefore := maps.Clone(s.memberships), maps.Clone(s.members)
	invitationsBefore, hashesBefore := maps.Clone(s.invitations), maps.Clone(s.tokenHashes)
	branchesBefore := maps.Clone(s.branches)
	auditBefore := len(s.audit)
	if err := fn(ctx, fakeTx{s}); err != nil {
		s.users, s.orgs, s.idem = usersBefore, orgsBefore, idemBefore
		s.memberships, s.members = membershipsBefore, membersBefore
		s.invitations, s.tokenHashes, s.branches = invitationsBefore, hashesBefore, branchesBefore
		s.audit = s.audit[:auditBefore]
		return err
	}
	return nil
}

type fakeTx struct{ s *store }

func (t fakeTx) Users() UserRepository                 { return fakeUsers(t) }
func (t fakeTx) Memberships() MembershipRepository     { return fakeMemberships(t) }
func (t fakeTx) Audit() AuditRecorder                  { return fakeAudit(t) }
func (t fakeTx) Organizations() OrganizationRepository { return fakeOrgs(t) }
func (t fakeTx) Idempotency() IdempotencyStore         { return fakeIdem(t) }
func (t fakeTx) Members() MemberRepository             { return fakeMembers(t) }

type fakeMembers struct{ s *store }

// List ordena por (JoinedAt, MembershipID) y aplica el cursor, como la consulta real.
func (r fakeMembers) List(_ context.Context, _ uuid.UUID, q MemberQuery) ([]membership.Member, error) {
	all := slices.Collect(maps.Values(r.s.members))
	slices.SortFunc(all, func(a, b membership.Member) int {
		if c := a.JoinedAt.Compare(b.JoinedAt); c != 0 {
			return c
		}
		return strings.Compare(a.MembershipID.String(), b.MembershipID.String())
	})
	var out []membership.Member
	for _, m := range all {
		if q.Status != "" && m.Status != q.Status {
			continue
		}
		if q.After != nil && (m.JoinedAt.Before(q.After.At) ||
			m.JoinedAt.Equal(q.After.At) && m.MembershipID.String() <= q.After.ID.String()) {
			continue
		}
		out = append(out, m)
		if len(out) == q.Limit {
			break
		}
	}
	return out, nil
}

func (r fakeMembers) GetForUpdate(_ context.Context, _ uuid.UUID, userID uuid.UUID) (membership.Member, error) {
	m, ok := r.s.members[userID]
	if !ok {
		return membership.Member{}, ErrNotFound
	}
	return m, nil
}

func (r fakeMembers) CountActiveOwners(context.Context, uuid.UUID) (int, error) {
	n := 0
	for _, m := range r.s.members {
		if m.IsActiveOwner() {
			n++
		}
	}
	return n, nil
}

func (r fakeMembers) ReplaceRoles(_ context.Context, _ uuid.UUID, membershipID uuid.UUID, roles []membership.Role, _ uuid.UUID) error {
	return r.update(membershipID, func(m *membership.Member) { m.Roles = slices.Clone(roles) })
}

func (r fakeMembers) SetStatus(_ context.Context, _ uuid.UUID, membershipID uuid.UUID, status membership.Status) error {
	return r.update(membershipID, func(m *membership.Member) { m.Status = status })
}

func (r fakeMembers) update(membershipID uuid.UUID, fn func(*membership.Member)) error {
	for id, m := range r.s.members {
		if m.MembershipID == membershipID {
			fn(&m)
			r.s.members[id] = m
			return nil
		}
	}
	return ErrNotFound
}

func (r fakeMembers) FindByUser(_ context.Context, _ uuid.UUID, userID uuid.UUID) (uuid.UUID, membership.Status, error) {
	m, ok := r.s.members[userID]
	if !ok {
		return uuid.Nil, "", ErrNotFound
	}
	return m.MembershipID, m.Status, nil
}

func (t fakeTx) Invitations() InvitationRepository { return fakeInvitations(t) }

type fakeInvitations struct{ s *store }

func (r fakeInvitations) Create(_ context.Context, inv invitation.Invitation, hash string) (invitation.Invitation, error) {
	for _, other := range r.s.invitations {
		if other.Email == inv.Email && other.Status == invitation.StatusPending {
			return invitation.Invitation{}, ErrConflict
		}
	}
	inv.ID, inv.CreatedAt = uuid.New(), time.Now()
	r.s.invitations[inv.ID] = inv
	r.s.tokenHashes[hash] = inv.ID
	return inv, nil
}

func (r fakeInvitations) List(_ context.Context, _ uuid.UUID, q InvitationQuery) ([]invitation.Invitation, error) {
	all := slices.Collect(maps.Values(r.s.invitations))
	slices.SortFunc(all, func(a, b invitation.Invitation) int { return a.CreatedAt.Compare(b.CreatedAt) })
	if len(all) > q.Limit {
		all = all[:q.Limit]
	}
	return all, nil
}

func (r fakeInvitations) GetForUpdate(_ context.Context, _ uuid.UUID, id uuid.UUID) (invitation.Invitation, error) {
	inv, ok := r.s.invitations[id]
	if !ok {
		return invitation.Invitation{}, ErrNotFound
	}
	return inv, nil
}

// FindByTokenHash imita la política invitations_invitee: solo ve invitaciones al email del usuario de la sesión.
func (r fakeInvitations) FindByTokenHash(_ context.Context, hash string) (invitation.Invitation, error) {
	inv, ok := r.s.invitations[r.s.tokenHashes[hash]]
	if !ok {
		return invitation.Invitation{}, ErrNotFound
	}
	sessionUser := r.s.txUsers[len(r.s.txUsers)-1]
	for _, u := range r.s.users {
		if u.ID == sessionUser && strings.EqualFold(u.Email, inv.Email) {
			return inv, nil
		}
	}
	return invitation.Invitation{}, ErrNotFound
}

func (r fakeInvitations) GetByTokenHashForUpdate(_ context.Context, _ uuid.UUID, hash string) (invitation.Invitation, error) {
	inv, ok := r.s.invitations[r.s.tokenHashes[hash]]
	if !ok {
		return invitation.Invitation{}, ErrNotFound
	}
	return inv, nil
}

func (r fakeInvitations) Revoke(_ context.Context, _ uuid.UUID, id uuid.UUID) error {
	inv := r.s.invitations[id]
	inv.Status = invitation.StatusRevoked
	r.s.invitations[id] = inv
	return nil
}

func (r fakeInvitations) MarkAccepted(_ context.Context, _ uuid.UUID, id, userID uuid.UUID) error {
	inv := r.s.invitations[id]
	inv.Status, inv.AcceptedByUserID = invitation.StatusAccepted, userID
	r.s.invitations[id] = inv
	return nil
}

func (r fakeInvitations) ExpireStalePending(context.Context, uuid.UUID, string) error { return nil }

func (r fakeInvitations) IsActiveMemberEmail(_ context.Context, _ uuid.UUID, email string) (bool, error) {
	for _, m := range r.s.members {
		if strings.EqualFold(m.Email, email) && m.Status == membership.StatusActive {
			return true, nil
		}
	}
	return false, nil
}

func (t fakeTx) Branches() BranchRepository { return fakeBranches(t) }

type fakeBranches struct{ s *store }

func (r fakeBranches) Create(_ context.Context, b branch.Branch) (branch.Branch, error) {
	for _, other := range r.s.branches {
		if other.OrganizationID == b.OrganizationID && other.Code == b.Code {
			return branch.Branch{}, ErrConflict
		}
	}
	b.ID, b.CreatedAt = uuid.New(), time.Now()
	r.s.branches[b.ID] = b
	return b, nil
}

func (r fakeBranches) List(_ context.Context, org uuid.UUID, q BranchQuery) ([]branch.Branch, error) {
	var out []branch.Branch
	for _, b := range r.s.branches {
		if b.OrganizationID == org && (q.Active == nil || b.IsActive == *q.Active) {
			out = append(out, b)
		}
	}
	slices.SortFunc(out, func(a, b branch.Branch) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out[:min(len(out), q.Limit)], nil
}

// Get imita RLS: una sucursal de otra organización no existe.
func (r fakeBranches) Get(_ context.Context, org, id uuid.UUID) (branch.Branch, error) {
	b, ok := r.s.branches[id]
	if !ok || b.OrganizationID != org {
		return branch.Branch{}, ErrNotFound
	}
	return b, nil
}

func (r fakeBranches) GetForUpdate(ctx context.Context, org, id uuid.UUID) (branch.Branch, error) {
	return r.Get(ctx, org, id)
}

func (r fakeBranches) Update(_ context.Context, b branch.Branch) (branch.Branch, error) {
	r.s.branches[b.ID] = b
	return b, nil
}

type fakeCache struct{ invalidated []string }

func (c *fakeCache) Invalidate(subject string, _ uuid.UUID) {
	c.invalidated = append(c.invalidated, subject)
}

type fakeOrgs struct{ s *store }

func (r fakeOrgs) Create(_ context.Context, o organization.Organization) (organization.Organization, error) {
	for _, other := range r.s.orgs {
		if other.IdentificationTypeCode == o.IdentificationTypeCode && other.IdentificationNumber == o.IdentificationNumber {
			return organization.Organization{}, ErrConflict
		}
	}
	o.DefaultCurrencyCode = "CRC"
	r.s.orgs[o.ID] = o
	return o, nil
}

func (r fakeOrgs) Get(_ context.Context, id uuid.UUID) (organization.Organization, error) {
	o, ok := r.s.orgs[id]
	if !ok {
		return organization.Organization{}, ErrNotFound
	}
	return o, nil
}

func (r fakeOrgs) GetForUpdate(ctx context.Context, id uuid.UUID) (organization.Organization, error) {
	return r.Get(ctx, id)
}

func (r fakeOrgs) Update(_ context.Context, o organization.Organization) (organization.Organization, error) {
	r.s.orgs[o.ID] = o
	return o, nil
}

type fakeIdem struct{ s *store }

func (f fakeIdem) Claim(_ context.Context, org uuid.UUID, key, hash string, _ time.Duration) (*IdempotencyRecord, error) {
	k := org.String() + "|" + key
	if rec, ok := f.s.idem[k]; ok {
		return &rec, nil
	}
	f.s.idem[k] = IdempotencyRecord{RequestHash: hash}
	return nil, nil
}

func (f fakeIdem) Complete(_ context.Context, org uuid.UUID, key string, rec IdempotencyRecord) error {
	f.s.idem[org.String()+"|"+key] = rec
	return nil
}

type fakeUsers struct{ s *store }

func (r fakeUsers) FindBySubject(_ context.Context, subject string) (user.User, error) {
	u, ok := r.s.users[subject]
	if !ok {
		return user.User{}, ErrNotFound
	}
	return u, nil
}

func (r fakeUsers) CreateIfAbsent(_ context.Context, u user.User) (user.User, bool, error) {
	if r.s.raceOnCreate {
		u.ID = uuid.New()
		r.s.users[u.Subject] = u
		return u, false, nil
	}
	if existing, ok := r.s.users[u.Subject]; ok {
		return existing, false, nil
	}
	u.ID = uuid.New()
	r.s.users[u.Subject] = u
	return u, true, nil
}

func (r fakeUsers) SetActiveOrganization(_ context.Context, userID, orgID uuid.UUID) error {
	for k, u := range r.s.users {
		if u.ID == userID {
			u.ActiveOrganizationID = orgID
			r.s.users[k] = u
			return nil
		}
	}
	return ErrNotFound
}

type fakeMemberships struct{ s *store }

func (r fakeMemberships) ListActiveForUser(_ context.Context, userID uuid.UUID) ([]membership.Summary, error) {
	return r.s.memberships[userID], nil
}

func (r fakeMemberships) Add(_ context.Context, orgID, userID uuid.UUID) (uuid.UUID, error) {
	r.s.memberships[userID] = append(r.s.memberships[userID], membership.Summary{OrganizationID: orgID, Status: membership.StatusActive})
	return uuid.New(), nil
}

func (r fakeMemberships) AssignRole(_ context.Context, orgID, _ uuid.UUID, role membership.Role, _ uuid.UUID) error {
	for userID, list := range r.s.memberships {
		for i := range list {
			if list[i].OrganizationID == orgID && len(list[i].Roles) == 0 {
				list[i].Roles = append(list[i].Roles, role)
				r.s.memberships[userID] = list
				return nil
			}
		}
	}
	return ErrNotFound
}

func (r fakeMemberships) IsActiveMember(_ context.Context, userID, orgID uuid.UUID) (bool, error) {
	for _, m := range r.s.memberships[userID] {
		if m.OrganizationID == orgID && m.Status == membership.StatusActive {
			return true, nil
		}
	}
	return false, nil
}

type fakeAudit struct{ s *store }

func (a fakeAudit) Record(_ context.Context, e AuditEvent) error {
	a.s.audit = append(a.s.audit, e)
	return nil
}
