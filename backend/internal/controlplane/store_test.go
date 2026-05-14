package controlplane

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuditEventsListIsImmutableCopy(t *testing.T) {
	store := NewStore()
	err := store.AppendAudit(context.Background(), AuditEvent{
		ID:         "audit_1",
		ActorID:    "actor_1",
		TenantID:   "tenant_1",
		Action:     "tenant.update",
		Resource:   "tenant",
		ResourceID: "tenant_1",
		OccurredAt: time.Now().UTC(),
	})
	require.NoError(t, err)

	events, err := store.ListAuditEvents(context.Background(), AuditFilter{})
	require.NoError(t, err)
	require.Len(t, events, 1)
	events[0].Action = "mutated"

	events2, err := store.ListAuditEvents(context.Background(), AuditFilter{})
	require.NoError(t, err)
	require.Len(t, events2, 1)
	require.Equal(t, "tenant.update", events2[0].Action)
}
