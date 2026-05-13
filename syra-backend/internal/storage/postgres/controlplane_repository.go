package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"syra-backend/internal/auth"
	"syra-backend/internal/billing"
	"syra-backend/internal/controlplane"
	"syra-backend/pkg/ids"
)

type queryer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type ControlPlaneRepository struct {
	pool *pgxpool.Pool
	db   queryer
}

const runtimeConfigScope = "gateway_runtime"

func NewControlPlaneRepository(pool *pgxpool.Pool) *ControlPlaneRepository {
	return &ControlPlaneRepository{pool: pool, db: pool}
}

func (r *ControlPlaneRepository) WithTx(ctx context.Context, fn func(*ControlPlaneRepository) error) error {
	if r.pool == nil {
		return fmt.Errorf("postgres pool is required for transactions")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	txRepo := &ControlPlaneRepository{pool: r.pool, db: tx}
	if err := fn(txRepo); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *ControlPlaneRepository) CreateBillingPlan(ctx context.Context, plan billing.BillingPlan) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO billing_plans (
			id, name, slug, monthly_fee, included_requests, overage_price, currency, status, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, plan.ID, plan.Name, plan.Slug, plan.MonthlyFee, plan.IncludedRequests, plan.OveragePrice, plan.Currency, plan.Status, time.Now().UTC(), time.Now().UTC())
	return err
}

func (r *ControlPlaneRepository) ListBillingPlans(ctx context.Context) ([]billing.BillingPlan, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, name, slug, monthly_fee, included_requests, overage_price, currency, status
		FROM billing_plans
		WHERE deleted_at IS NULL
		ORDER BY created_at, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var plans []billing.BillingPlan
	for rows.Next() {
		var p billing.BillingPlan
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.MonthlyFee, &p.IncludedRequests, &p.OveragePrice, &p.Currency, &p.Status); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

func (r *ControlPlaneRepository) GetBillingPlan(ctx context.Context, id string) (billing.BillingPlan, error) {
	var p billing.BillingPlan
	err := r.db.QueryRow(ctx, `
		SELECT id::text, name, slug, monthly_fee, included_requests, overage_price, currency, status
		FROM billing_plans
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&p.ID, &p.Name, &p.Slug, &p.MonthlyFee, &p.IncludedRequests, &p.OveragePrice, &p.Currency, &p.Status)
	if err != nil {
		return billing.BillingPlan{}, mapNotFound(err)
	}
	return p, nil
}

func (r *ControlPlaneRepository) UpdateBillingPlan(ctx context.Context, plan billing.BillingPlan) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE billing_plans
		SET name = $2, slug = $3, monthly_fee = $4, included_requests = $5, overage_price = $6, currency = $7, status = $8, updated_at = $9
		WHERE id = $1 AND deleted_at IS NULL
	`, plan.ID, plan.Name, plan.Slug, plan.MonthlyFee, plan.IncludedRequests, plan.OveragePrice, plan.Currency, plan.Status, time.Now().UTC())
	return rowsAffected(tag, err)
}

func (r *ControlPlaneRepository) UpsertBillingSummary(ctx context.Context, summary billing.BillingSummary) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO billing_summaries (
			id, tenant_id, billing_period, billing_plan_id, request_count, success_count, failure_count,
			rejected_count, timeout_count, billable_count, included_quota, billable_overage, monthly_fee,
			overage_amount, estimated_amount, currency, status, calculated_at, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20
		)
		ON CONFLICT (tenant_id, billing_period) DO UPDATE SET
			billing_plan_id = EXCLUDED.billing_plan_id,
			request_count = EXCLUDED.request_count,
			success_count = EXCLUDED.success_count,
			failure_count = EXCLUDED.failure_count,
			rejected_count = EXCLUDED.rejected_count,
			timeout_count = EXCLUDED.timeout_count,
			billable_count = EXCLUDED.billable_count,
			included_quota = EXCLUDED.included_quota,
			billable_overage = EXCLUDED.billable_overage,
			monthly_fee = EXCLUDED.monthly_fee,
			overage_amount = EXCLUDED.overage_amount,
			estimated_amount = EXCLUDED.estimated_amount,
			currency = EXCLUDED.currency,
			status = EXCLUDED.status,
			calculated_at = EXCLUDED.calculated_at,
			updated_at = EXCLUDED.updated_at
	`, ids.New(), summary.TenantID, summary.BillingPeriod, nullableString(summary.PlanID), summary.TotalRequests, summary.TotalRequests-summary.FailedRequests-summary.RejectedRequests-summary.TimeoutRequests, summary.FailedRequests, summary.RejectedRequests, summary.TimeoutRequests, summary.BillableRequests, summary.IncludedRequests, summary.OverageRequests, summary.MonthlyFee, summary.OverageAmount, summary.EstimatedAmount, summary.Currency, summary.Status, summary.CalculatedAt, time.Now().UTC(), time.Now().UTC())
	return err
}

func (r *ControlPlaneRepository) GetBillingSummary(ctx context.Context, tenantID, billingPeriod string) (billing.BillingSummary, error) {
	var s billing.BillingSummary
	err := r.db.QueryRow(ctx, `
		SELECT tenant_id::text, billing_period, COALESCE(billing_plan_id::text,''), request_count, failure_count, rejected_count, timeout_count,
			billable_count, included_quota, billable_overage, monthly_fee, overage_amount, estimated_amount, currency, status, calculated_at
		FROM billing_summaries
		WHERE tenant_id = $1 AND billing_period = $2
	`, tenantID, billingPeriod).Scan(
		&s.TenantID, &s.BillingPeriod, &s.PlanID, &s.TotalRequests, &s.FailedRequests, &s.RejectedRequests, &s.TimeoutRequests,
		&s.BillableRequests, &s.IncludedRequests, &s.OverageRequests, &s.MonthlyFee, &s.OverageAmount, &s.EstimatedAmount, &s.Currency, &s.Status, &s.CalculatedAt,
	)
	if err != nil {
		return billing.BillingSummary{}, mapNotFound(err)
	}
	return s, nil
}

func (r *ControlPlaneRepository) CreateTenant(ctx context.Context, tenant controlplane.Tenant) error {
	metadata := jsonOrEmptyObject(tenant.Metadata)
	_, err := r.db.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, status, billing_plan_id, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, tenant.ID, tenant.Name, tenant.Slug, tenant.Status, nullableString(tenant.BillingPlanID), metadata, tenant.CreatedAt, tenant.UpdatedAt)
	if err != nil {
		return err
	}
	return r.bumpRuntimeConfigVersion(ctx)
}

func (r *ControlPlaneRepository) ListTenants(ctx context.Context) ([]controlplane.Tenant, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, name, slug, status, COALESCE(billing_plan_id::text, ''), metadata, created_at, updated_at
		FROM tenants
		ORDER BY created_at, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectRows(rows, scanTenant)
}

func (r *ControlPlaneRepository) GetTenant(ctx context.Context, tenantID string) (controlplane.Tenant, error) {
	return scanTenant(r.db.QueryRow(ctx, `
		SELECT id::text, name, slug, status, COALESCE(billing_plan_id::text, ''), metadata, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`, tenantID))
}

func (r *ControlPlaneRepository) UpdateTenant(ctx context.Context, tenant controlplane.Tenant) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE tenants
		SET name = $2, slug = $3, status = $4, billing_plan_id = $5, metadata = $6, updated_at = $7
		WHERE id = $1
	`, tenant.ID, tenant.Name, tenant.Slug, tenant.Status, nullableString(tenant.BillingPlanID), jsonOrEmptyObject(tenant.Metadata), tenant.UpdatedAt)
	if err := rowsAffected(tag, err); err != nil {
		return err
	}
	return r.bumpRuntimeConfigVersion(ctx)
}

func (r *ControlPlaneRepository) CreateAPIProduct(ctx context.Context, product controlplane.APIProduct) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO api_products (id, tenant_id, name, slug, description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, product.ID, product.TenantID, product.Name, product.Slug, nullableString(product.Description), product.Status, product.CreatedAt, product.UpdatedAt)
	if err != nil {
		return err
	}
	return r.bumpRuntimeConfigVersion(ctx)
}

func (r *ControlPlaneRepository) ListAPIProducts(ctx context.Context, tenantID string) ([]controlplane.APIProduct, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, tenant_id::text, name, slug, COALESCE(description, ''), status, created_at, updated_at
		FROM api_products
		WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY created_at, id
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectRows(rows, scanAPIProduct)
}

func (r *ControlPlaneRepository) GetAPIProduct(ctx context.Context, tenantID, id string) (controlplane.APIProduct, error) {
	return scanAPIProduct(r.db.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, name, slug, COALESCE(description, ''), status, created_at, updated_at
		FROM api_products
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	`, tenantID, id))
}

func (r *ControlPlaneRepository) UpdateAPIProduct(ctx context.Context, product controlplane.APIProduct) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE api_products
		SET name = $3, slug = $4, description = $5, status = $6, updated_at = $7
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	`, product.TenantID, product.ID, product.Name, product.Slug, nullableString(product.Description), product.Status, product.UpdatedAt)
	if err := rowsAffected(tag, err); err != nil {
		return err
	}
	return r.bumpRuntimeConfigVersion(ctx)
}

func (r *ControlPlaneRepository) CreateUpstream(ctx context.Context, upstream controlplane.Upstream) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO upstreams (id, tenant_id, name, protocol, config, secret_ref, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, upstream.ID, upstream.TenantID, upstream.Name, upstream.Protocol, jsonOrEmptyRaw(upstream.Config), upstream.SecretRef, upstream.Status, upstream.CreatedAt, upstream.UpdatedAt)
	if err != nil {
		return err
	}
	return r.bumpRuntimeConfigVersion(ctx)
}

func (r *ControlPlaneRepository) ListUpstreams(ctx context.Context, tenantID string) ([]controlplane.Upstream, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, tenant_id::text, name, protocol, config, COALESCE(secret_ref, ''), status, created_at, updated_at
		FROM upstreams
		WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY created_at, id
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectRows(rows, scanUpstream)
}

func (r *ControlPlaneRepository) GetUpstream(ctx context.Context, tenantID, id string) (controlplane.Upstream, error) {
	return scanUpstream(r.db.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, name, protocol, config, COALESCE(secret_ref, ''), status, created_at, updated_at
		FROM upstreams
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	`, tenantID, id))
}

func (r *ControlPlaneRepository) UpdateUpstream(ctx context.Context, upstream controlplane.Upstream) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE upstreams
		SET name = $3, protocol = $4, config = $5, secret_ref = $6, status = $7, updated_at = $8
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	`, upstream.TenantID, upstream.ID, upstream.Name, upstream.Protocol, jsonOrEmptyRaw(upstream.Config), upstream.SecretRef, upstream.Status, upstream.UpdatedAt)
	if err := rowsAffected(tag, err); err != nil {
		return err
	}
	return r.bumpRuntimeConfigVersion(ctx)
}

func (r *ControlPlaneRepository) CreateRoute(ctx context.Context, route controlplane.Route) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO routes (
			id, tenant_id, api_product_id, name, inbound_protocol, outbound_protocol, host, method, path,
			upstream_id, transformation_template_id, rate_limit_policy_id, quota_policy_id, priority, timeout_ms,
			required_scopes, hmac_enabled, hmac_secret, replay_window_sec, idempotency_enabled, idempotency_ttl_sec,
			status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
	`, route.ID, route.TenantID, route.APIProductID, route.Name, route.InboundProtocol, route.OutboundProtocol, nullableString(route.Host), nullableString(route.Method), nullableString(route.Path), route.UpstreamID, nullableString(route.TransformationTemplateID), nullableString(route.RateLimitPolicyID), nullableString(route.QuotaPolicyID), route.Priority, route.TimeoutMs, route.RequiredScopes, route.HMACEnabled, nullableString(route.HMACSecret), route.ReplayWindowSec, route.IdempotencyEnabled, route.IdempotencyTTLSec, route.Status, route.CreatedAt, route.UpdatedAt)
	if err != nil {
		return err
	}
	return r.bumpRuntimeConfigVersion(ctx)
}

func (r *ControlPlaneRepository) ListRoutes(ctx context.Context, tenantID string) ([]controlplane.Route, error) {
	rows, err := r.db.Query(ctx, routeSelectSQL()+`
		WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY priority, created_at, id
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectRows(rows, scanRoute)
}

func (r *ControlPlaneRepository) GetRoute(ctx context.Context, tenantID, id string) (controlplane.Route, error) {
	return scanRoute(r.db.QueryRow(ctx, routeSelectSQL()+`
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	`, tenantID, id))
}

func (r *ControlPlaneRepository) UpdateRoute(ctx context.Context, route controlplane.Route) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE routes
		SET api_product_id = $3, name = $4, inbound_protocol = $5, outbound_protocol = $6, host = $7, method = $8,
			path = $9, upstream_id = $10, transformation_template_id = $11, rate_limit_policy_id = $12,
			quota_policy_id = $13, priority = $14, timeout_ms = $15, required_scopes = $16, hmac_enabled = $17,
			hmac_secret = $18, replay_window_sec = $19, idempotency_enabled = $20, idempotency_ttl_sec = $21,
			status = $22, updated_at = $23
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	`, route.TenantID, route.ID, route.APIProductID, route.Name, route.InboundProtocol, route.OutboundProtocol, nullableString(route.Host), nullableString(route.Method), nullableString(route.Path), route.UpstreamID, nullableString(route.TransformationTemplateID), nullableString(route.RateLimitPolicyID), nullableString(route.QuotaPolicyID), route.Priority, route.TimeoutMs, route.RequiredScopes, route.HMACEnabled, nullableString(route.HMACSecret), route.ReplayWindowSec, route.IdempotencyEnabled, route.IdempotencyTTLSec, route.Status, route.UpdatedAt)
	if err := rowsAffected(tag, err); err != nil {
		return err
	}
	return r.bumpRuntimeConfigVersion(ctx)
}

func (r *ControlPlaneRepository) CreateConsumer(ctx context.Context, consumer controlplane.Consumer) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO consumers (id, tenant_id, name, slug, owner_user_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, consumer.ID, consumer.TenantID, consumer.Name, consumer.Slug, nullableString(consumer.OwnerUserID), consumer.Status, consumer.CreatedAt, consumer.UpdatedAt)
	return err
}

func (r *ControlPlaneRepository) GetConsumer(ctx context.Context, tenantID, id string) (controlplane.Consumer, error) {
	return scanConsumer(r.db.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, name, slug, COALESCE(owner_user_id::text, ''), status, created_at, updated_at
		FROM consumers
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	`, tenantID, id))
}

func (r *ControlPlaneRepository) CreateCredential(ctx context.Context, credential controlplane.Credential) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO credentials (id, tenant_id, consumer_id, type, key_prefix, secret_hash, scopes, status, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, credential.ID, credential.TenantID, credential.ConsumerID, credential.Type, nullableString(credential.KeyPrefix), nullableString(credential.SecretHash), credential.Scopes, credential.Status, credential.ExpiresAt, credential.CreatedAt, credential.UpdatedAt)
	if err != nil {
		return err
	}
	return r.bumpRuntimeConfigVersion(ctx)
}

func (r *ControlPlaneRepository) GetCredential(ctx context.Context, tenantID, id string) (controlplane.Credential, error) {
	return scanCredential(r.db.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, consumer_id::text, type, COALESCE(key_prefix, ''), COALESCE(secret_hash, ''), scopes, status, expires_at, created_at, updated_at
		FROM credentials
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id))
}

func (r *ControlPlaneRepository) UpdateCredential(ctx context.Context, credential controlplane.Credential) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE credentials
		SET key_prefix = $3, secret_hash = $4, scopes = $5, status = $6, expires_at = $7, updated_at = $8
		WHERE tenant_id = $1 AND id = $2
	`, credential.TenantID, credential.ID, nullableString(credential.KeyPrefix), nullableString(credential.SecretHash), credential.Scopes, credential.Status, credential.ExpiresAt, credential.UpdatedAt)
	if err := rowsAffected(tag, err); err != nil {
		return err
	}
	return r.bumpRuntimeConfigVersion(ctx)
}

func (r *ControlPlaneRepository) FindByPrefix(ctx context.Context, prefix string) (auth.APIKeyCredential, error) {
	var credential auth.APIKeyCredential
	err := r.db.QueryRow(ctx, `
		SELECT c.id::text, c.tenant_id::text, c.consumer_id::text, COALESCE(c.key_prefix, ''), COALESCE(c.secret_hash, ''), c.status,
			COALESCE(t.status, ''), COALESCE(cons.status, ''), c.scopes, c.expires_at
		FROM credentials c
		LEFT JOIN tenants t ON t.id = c.tenant_id
		LEFT JOIN consumers cons ON cons.id = c.consumer_id
		WHERE c.key_prefix = $1
	`, prefix).Scan(&credential.ID, &credential.TenantID, &credential.ConsumerID, &credential.KeyPrefix, &credential.SecretHash, &credential.Status, &credential.TenantStatus, &credential.ConsumerStatus, &credential.Scopes, &credential.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.APIKeyCredential{}, auth.ErrCredentialNotFound
	}
	if err != nil {
		return auth.APIKeyCredential{}, err
	}
	return credential, nil
}

func (r *ControlPlaneRepository) CreateTemplate(ctx context.Context, template controlplane.TransformationTemplate) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO transformation_templates (
			id, tenant_id, api_product_id, name, source_protocol, target_protocol, version, template_body,
			status, published_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, template.ID, template.TenantID, nullableString(template.APIProductID), template.Name, template.SourceProtocol, template.TargetProtocol, template.Version, jsonOrEmptyRaw(template.TemplateBody), template.Status, template.PublishedAt, template.CreatedAt, template.UpdatedAt)
	if err != nil {
		return err
	}
	return r.bumpRuntimeConfigVersion(ctx)
}

func (r *ControlPlaneRepository) ListTemplates(ctx context.Context, tenantID string) ([]controlplane.TransformationTemplate, error) {
	rows, err := r.db.Query(ctx, templateSelectSQL()+`
		WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY created_at, id
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectRows(rows, scanTemplate)
}

func (r *ControlPlaneRepository) GetTemplate(ctx context.Context, tenantID, id string) (controlplane.TransformationTemplate, error) {
	return scanTemplate(r.db.QueryRow(ctx, templateSelectSQL()+`
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	`, tenantID, id))
}

func (r *ControlPlaneRepository) UpdateTemplate(ctx context.Context, template controlplane.TransformationTemplate) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE transformation_templates
		SET api_product_id = $3, name = $4, source_protocol = $5, target_protocol = $6, version = $7,
			template_body = $8, status = $9, published_at = $10, updated_at = $11
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	`, template.TenantID, template.ID, nullableString(template.APIProductID), template.Name, template.SourceProtocol, template.TargetProtocol, template.Version, jsonOrEmptyRaw(template.TemplateBody), template.Status, template.PublishedAt, template.UpdatedAt)
	if err := rowsAffected(tag, err); err != nil {
		return err
	}
	return r.bumpRuntimeConfigVersion(ctx)
}

func (r *ControlPlaneRepository) AppendAudit(ctx context.Context, event controlplane.AuditEvent) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO audit_logs (id, tenant_id, actor_role, action, resource_type, resource_id, after, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, event.ID, nullableString(event.TenantID), nullableString(event.ActorID), event.Action, event.Resource, event.ResourceID, nullableRaw(event.Metadata), event.OccurredAt)
	return err
}

func (r *ControlPlaneRepository) ListAuditEvents(ctx context.Context, filter controlplane.AuditFilter) ([]controlplane.AuditEvent, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, COALESCE(tenant_id::text, ''), COALESCE(actor_user_id::text, actor_role, ''),
			action, resource_type, resource_id, COALESCE(after, '{}'::jsonb), occurred_at
		FROM audit_logs
		WHERE ($1 = '' OR tenant_id = NULLIF($1, '')::uuid)
			AND ($2 = '' OR COALESCE(actor_user_id::text, actor_role, '') = $2)
			AND ($3 = '' OR action = $3)
			AND ($4 = '' OR resource_type = $4)
			AND ($5::timestamptz IS NULL OR occurred_at >= $5)
			AND ($6::timestamptz IS NULL OR occurred_at < $6)
		ORDER BY occurred_at, id
	`, filter.TenantID, filter.ActorID, filter.Action, filter.Resource, filter.From, filter.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectRows(rows, scanAuditEvent)
}

func routeSelectSQL() string {
	return `
		SELECT id::text, tenant_id::text, api_product_id::text, name, inbound_protocol, outbound_protocol,
			COALESCE(host, ''), COALESCE(method, ''), COALESCE(path, ''), upstream_id::text,
			COALESCE(transformation_template_id::text, ''), COALESCE(rate_limit_policy_id::text, ''),
			COALESCE(quota_policy_id::text, ''), priority, timeout_ms, required_scopes, hmac_enabled,
			COALESCE(hmac_secret, ''), replay_window_sec, idempotency_enabled, idempotency_ttl_sec, status, created_at, updated_at
		FROM routes
	`
}

func templateSelectSQL() string {
	return `
		SELECT id::text, tenant_id::text, COALESCE(api_product_id::text, ''), name, source_protocol, target_protocol,
			template_body, version, status, published_at, created_at, updated_at
		FROM transformation_templates
	`
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTenant(row rowScanner) (controlplane.Tenant, error) {
	var tenant controlplane.Tenant
	var metadata []byte
	if err := row.Scan(&tenant.ID, &tenant.Name, &tenant.Slug, &tenant.Status, &tenant.BillingPlanID, &metadata, &tenant.CreatedAt, &tenant.UpdatedAt); err != nil {
		return controlplane.Tenant{}, mapNotFound(err)
	}
	_ = json.Unmarshal(metadata, &tenant.Metadata)
	return tenant, nil
}

func scanAPIProduct(row rowScanner) (controlplane.APIProduct, error) {
	var product controlplane.APIProduct
	if err := row.Scan(&product.ID, &product.TenantID, &product.Name, &product.Slug, &product.Description, &product.Status, &product.CreatedAt, &product.UpdatedAt); err != nil {
		return controlplane.APIProduct{}, mapNotFound(err)
	}
	return product, nil
}

func scanUpstream(row rowScanner) (controlplane.Upstream, error) {
	var item controlplane.Upstream
	var secretRef string
	if err := row.Scan(&item.ID, &item.TenantID, &item.Name, &item.Protocol, &item.Config, &secretRef, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return controlplane.Upstream{}, mapNotFound(err)
	}
	item.SecretRef = stringPtr(secretRef)
	return item, nil
}

func scanRoute(row rowScanner) (controlplane.Route, error) {
	var route controlplane.Route
	if err := row.Scan(
		&route.ID, &route.TenantID, &route.APIProductID, &route.Name, &route.InboundProtocol, &route.OutboundProtocol,
		&route.Host, &route.Method, &route.Path, &route.UpstreamID, &route.TransformationTemplateID,
		&route.RateLimitPolicyID, &route.QuotaPolicyID, &route.Priority, &route.TimeoutMs, &route.RequiredScopes,
		&route.HMACEnabled, &route.HMACSecret, &route.ReplayWindowSec, &route.IdempotencyEnabled, &route.IdempotencyTTLSec, &route.Status,
		&route.CreatedAt, &route.UpdatedAt,
	); err != nil {
		return controlplane.Route{}, mapNotFound(err)
	}
	return route, nil
}

func scanConsumer(row rowScanner) (controlplane.Consumer, error) {
	var consumer controlplane.Consumer
	if err := row.Scan(&consumer.ID, &consumer.TenantID, &consumer.Name, &consumer.Slug, &consumer.OwnerUserID, &consumer.Status, &consumer.CreatedAt, &consumer.UpdatedAt); err != nil {
		return controlplane.Consumer{}, mapNotFound(err)
	}
	return consumer, nil
}

func scanCredential(row rowScanner) (controlplane.Credential, error) {
	var credential controlplane.Credential
	if err := row.Scan(&credential.ID, &credential.TenantID, &credential.ConsumerID, &credential.Type, &credential.KeyPrefix, &credential.SecretHash, &credential.Scopes, &credential.Status, &credential.ExpiresAt, &credential.CreatedAt, &credential.UpdatedAt); err != nil {
		return controlplane.Credential{}, mapNotFound(err)
	}
	return credential, nil
}

func scanTemplate(row rowScanner) (controlplane.TransformationTemplate, error) {
	var template controlplane.TransformationTemplate
	if err := row.Scan(&template.ID, &template.TenantID, &template.APIProductID, &template.Name, &template.SourceProtocol, &template.TargetProtocol, &template.TemplateBody, &template.Version, &template.Status, &template.PublishedAt, &template.CreatedAt, &template.UpdatedAt); err != nil {
		return controlplane.TransformationTemplate{}, mapNotFound(err)
	}
	return template, nil
}

func scanAuditEvent(row rowScanner) (controlplane.AuditEvent, error) {
	var event controlplane.AuditEvent
	if err := row.Scan(&event.ID, &event.TenantID, &event.ActorID, &event.Action, &event.Resource, &event.ResourceID, &event.Metadata, &event.OccurredAt); err != nil {
		return controlplane.AuditEvent{}, mapNotFound(err)
	}
	return event, nil
}

func collectRows[T any](rows pgx.Rows, scan func(rowScanner) (T, error)) ([]T, error) {
	items := []T{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ControlPlaneRepository) bumpRuntimeConfigVersion(ctx context.Context) error {
	var nextVersion int64
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM config_versions
		WHERE tenant_id IS NULL AND scope = $1
	`, runtimeConfigScope).Scan(&nextVersion); err != nil {
		return err
	}
	checksum := fmt.Sprintf("gateway-runtime-v%d", nextVersion)
	_, err := r.db.Exec(ctx, `
		INSERT INTO config_versions (id, tenant_id, scope, version, checksum, status, published_at, created_at)
		VALUES ($1, NULL, $2, $3, $4, 'active', $5, $5)
	`, ids.New(), runtimeConfigScope, nextVersion, checksum, time.Now().UTC())
	return err
}

func rowsAffected(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return controlplane.ErrNotFound
	}
	return nil
}

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return controlplane.ErrNotFound
	}
	return err
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func jsonOrEmptyObject(value map[string]any) string {
	if value == nil {
		return `{}`
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return `{}`
	}
	return string(encoded)
}

func jsonOrEmptyRaw(value json.RawMessage) string {
	if len(value) == 0 {
		return `{}`
	}
	return string(value)
}

func nullableRaw(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}

var _ controlplane.Repository = (*ControlPlaneRepository)(nil)
var _ auth.CredentialStore = (*ControlPlaneRepository)(nil)
