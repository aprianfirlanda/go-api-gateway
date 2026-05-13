package controlplane

import "context"
import "syra-backend/internal/billing"

type Repository interface {
	CreateBillingPlan(ctx context.Context, plan billing.BillingPlan) error
	ListBillingPlans(ctx context.Context) ([]billing.BillingPlan, error)
	GetBillingPlan(ctx context.Context, id string) (billing.BillingPlan, error)
	UpdateBillingPlan(ctx context.Context, plan billing.BillingPlan) error

	UpsertBillingSummary(ctx context.Context, summary billing.BillingSummary) error
	GetBillingSummary(ctx context.Context, tenantID, billingPeriod string) (billing.BillingSummary, error)

	CreateTenant(ctx context.Context, tenant Tenant) error
	ListTenants(ctx context.Context) ([]Tenant, error)
	GetTenant(ctx context.Context, tenantID string) (Tenant, error)
	UpdateTenant(ctx context.Context, tenant Tenant) error

	CreateAPIProduct(ctx context.Context, product APIProduct) error
	ListAPIProducts(ctx context.Context, tenantID string) ([]APIProduct, error)
	GetAPIProduct(ctx context.Context, tenantID, id string) (APIProduct, error)
	UpdateAPIProduct(ctx context.Context, product APIProduct) error

	CreateUpstream(ctx context.Context, upstream Upstream) error
	ListUpstreams(ctx context.Context, tenantID string) ([]Upstream, error)
	GetUpstream(ctx context.Context, tenantID, id string) (Upstream, error)
	UpdateUpstream(ctx context.Context, upstream Upstream) error

	CreateRoute(ctx context.Context, route Route) error
	ListRoutes(ctx context.Context, tenantID string) ([]Route, error)
	GetRoute(ctx context.Context, tenantID, id string) (Route, error)
	UpdateRoute(ctx context.Context, route Route) error

	CreateConsumer(ctx context.Context, consumer Consumer) error
	GetConsumer(ctx context.Context, tenantID, id string) (Consumer, error)

	CreateCredential(ctx context.Context, credential Credential) error
	GetCredential(ctx context.Context, tenantID, id string) (Credential, error)
	UpdateCredential(ctx context.Context, credential Credential) error

	CreateTemplate(ctx context.Context, template TransformationTemplate) error
	ListTemplates(ctx context.Context, tenantID string) ([]TransformationTemplate, error)
	GetTemplate(ctx context.Context, tenantID, id string) (TransformationTemplate, error)
	UpdateTemplate(ctx context.Context, template TransformationTemplate) error

	AppendAudit(ctx context.Context, event AuditEvent) error
	ListAuditEvents(ctx context.Context) ([]AuditEvent, error)
}
