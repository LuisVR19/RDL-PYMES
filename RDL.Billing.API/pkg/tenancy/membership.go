package tenancy

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Membership es el resultado de revalidar la membresía contra la base.
type Membership struct {
	UserID uuid.UUID
	Roles  []string
}

// MembershipResolver revalida que el sujeto siga siendo miembro activo de una organización activa.
// Devuelve ErrNoMembership si no lo es.
type MembershipResolver interface {
	ActiveMembership(ctx context.Context, subject string, organizationID uuid.UUID) (Membership, error)
}

// CachedResolver cachea solo resultados positivos durante ttl. Así una revocación surte efecto en ≤ ttl y
// una membresía recién creada (invitación aceptada) funciona de inmediato.
type CachedResolver struct {
	next MembershipResolver
	ttl  time.Duration
	now  func() time.Time

	mu      sync.Mutex
	entries map[cacheKey]cacheEntry
}

type cacheKey struct {
	subject string
	org     uuid.UUID
}

type cacheEntry struct {
	membership Membership
	expires    time.Time
}

func NewCachedResolver(next MembershipResolver, ttl time.Duration) *CachedResolver {
	return &CachedResolver{next: next, ttl: ttl, now: time.Now, entries: make(map[cacheKey]cacheEntry)}
}

func (c *CachedResolver) ActiveMembership(ctx context.Context, subject string, org uuid.UUID) (Membership, error) {
	key := cacheKey{subject: subject, org: org}
	now := c.now()

	c.mu.Lock()
	if e, ok := c.entries[key]; ok && now.Before(e.expires) {
		c.mu.Unlock()
		return copyMembership(e.membership), nil
	}
	c.mu.Unlock()

	m, err := c.next.ActiveMembership(ctx, subject, org)
	if err != nil {
		c.Invalidate(subject, org)
		return Membership{}, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictExpired(now)
	c.entries[key] = cacheEntry{membership: copyMembership(m), expires: now.Add(c.ttl)}
	return copyMembership(m), nil
}

// Invalidate descarta la entrada: lo usan los casos de uso que cambian rol o estado de una membresía,
// para que el propio nodo no espere al TTL.
func (c *CachedResolver) Invalidate(subject string, org uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, cacheKey{subject: subject, org: org})
}

func (c *CachedResolver) evictExpired(now time.Time) {
	for k, e := range c.entries {
		if !now.Before(e.expires) {
			delete(c.entries, k)
		}
	}
}

func copyMembership(m Membership) Membership {
	return Membership{UserID: m.UserID, Roles: slices.Clone(m.Roles)}
}
