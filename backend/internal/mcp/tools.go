package mcp

import (
	"context"
	"encoding/json"
	"net/http"

	"backend/internal/controlplane"
	"backend/pkg/ids"
)

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

func (s *server) canAccessTenant(identity Identity, tenantID string) bool {
	if identity.Role == "platform_admin" {
		return true
	}
	return identity.Role == "tenant_admin" && identity.TenantID != "" && identity.TenantID == tenantID
}

func (s *server) audit(ctx context.Context, identity Identity, tenantID, action, resource, resourceID string) {
	_ = s.store.AppendAudit(context.WithoutCancel(ctx), controlplane.AuditEvent{
		ID:         ids.New(),
		ActorID:    identity.ActorID,
		TenantID:   tenantID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		OccurredAt: s.now(),
	})
}
