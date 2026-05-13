package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"syra-backend/internal/auth"
	"syra-backend/internal/gateway/route"
	"syra-backend/internal/gateway/runtimeconfig"
	"syra-backend/internal/gateway/upstream"
	"syra-backend/internal/protocol/iso8583"
	restprotocol "syra-backend/internal/protocol/rest"
	"syra-backend/internal/protocol/soapxml"
	"syra-backend/internal/transform"
)

type RuntimeConfigSource struct {
	pool *pgxpool.Pool
}

func NewRuntimeConfigSource(pool *pgxpool.Pool) *RuntimeConfigSource {
	return &RuntimeConfigSource{pool: pool}
}

func (s *RuntimeConfigSource) Load(ctx context.Context) (runtimeconfig.Snapshot, error) {
	if s.pool == nil {
		return runtimeconfig.Snapshot{}, fmt.Errorf("postgres pool is required")
	}
	version, checksum, publishedAt, err := s.loadVersion(ctx)
	if err != nil {
		return runtimeconfig.Snapshot{}, err
	}
	tenantSet, err := s.loadActiveTenantSet(ctx)
	if err != nil {
		return runtimeconfig.Snapshot{}, err
	}
	productSet, err := s.loadActiveProductSet(ctx, tenantSet)
	if err != nil {
		return runtimeconfig.Snapshot{}, err
	}
	upstreams, err := s.loadActiveUpstreams(ctx, tenantSet)
	if err != nil {
		return runtimeconfig.Snapshot{}, err
	}
	upstreamSet := make(map[string]struct{}, len(upstreams))
	for _, item := range upstreams {
		upstreamSet[item.TenantID+"/"+item.ID] = struct{}{}
	}
	templates, templateSet, err := s.loadPublishedTemplates(ctx, tenantSet)
	if err != nil {
		return runtimeconfig.Snapshot{}, err
	}
	routes, err := s.loadActiveRoutes(ctx, tenantSet, productSet, upstreamSet, templateSet)
	if err != nil {
		return runtimeconfig.Snapshot{}, err
	}
	credentials, err := s.loadActiveCredentials(ctx, tenantSet)
	if err != nil {
		return runtimeconfig.Snapshot{}, err
	}
	return runtimeconfig.Snapshot{
		Version:     version,
		Checksum:    checksum,
		Status:      "active",
		PublishedAt: publishedAt,
		Routes:      routes,
		Upstreams:   upstreams,
		Credentials: credentials,
		Templates:   templates,
		Profiles:    []iso8583.Profile{},
	}, nil
}

func (s *RuntimeConfigSource) loadVersion(ctx context.Context) (int64, string, time.Time, error) {
	var version int64
	var checksum string
	var publishedAt time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT version, checksum, COALESCE(published_at, created_at)
		FROM config_versions
		WHERE tenant_id IS NULL AND scope = $1 AND status = 'active'
		ORDER BY version DESC
		LIMIT 1
	`, runtimeConfigScope).Scan(&version, &checksum, &publishedAt)
	if err == nil {
		return version, checksum, publishedAt.UTC(), nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return 1, "gateway-runtime-v1", time.Now().UTC(), nil
	}
	return 0, "", time.Time{}, err
}

func (s *RuntimeConfigSource) loadActiveTenantSet(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text FROM tenants WHERE status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tenants := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		tenants[id] = struct{}{}
	}
	return tenants, rows.Err()
}

func (s *RuntimeConfigSource) loadActiveProductSet(ctx context.Context, tenantSet map[string]struct{}) (map[string]struct{}, error) {
	rows, err := s.pool.Query(ctx, `SELECT tenant_id::text, id::text FROM api_products WHERE status = 'active' AND deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var tenantID, id string
		if err := rows.Scan(&tenantID, &id); err != nil {
			return nil, err
		}
		if _, ok := tenantSet[tenantID]; !ok {
			continue
		}
		out[tenantID+"/"+id] = struct{}{}
	}
	return out, rows.Err()
}

func (s *RuntimeConfigSource) loadActiveUpstreams(ctx context.Context, tenantSet map[string]struct{}) ([]upstream.Upstream, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT tenant_id::text, id::text, name, protocol, config
		FROM upstreams
		WHERE status = 'active' AND deleted_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []upstream.Upstream
	for rows.Next() {
		var tenantID, id, name, protocolName string
		var rawConfig []byte
		if err := rows.Scan(&tenantID, &id, &name, &protocolName, &rawConfig); err != nil {
			return nil, err
		}
		if _, ok := tenantSet[tenantID]; !ok {
			continue
		}
		item, err := convertUpstream(tenantID, id, name, protocolName, rawConfig)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *RuntimeConfigSource) loadPublishedTemplates(ctx context.Context, tenantSet map[string]struct{}) ([]transform.Template, map[string]struct{}, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT tenant_id::text, id::text, COALESCE(api_product_id::text, ''), name, source_protocol, target_protocol,
			version, template_body, status, published_at, created_at, updated_at
		FROM transformation_templates
		WHERE status = 'published' AND deleted_at IS NULL
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var items []transform.Template
	templateSet := map[string]struct{}{}
	for rows.Next() {
		var (
			tenantID, id, productID, name, source, target, status string
			version                                               int
			rawBody                                               []byte
			publishedAt                                           *time.Time
			createdAt                                             time.Time
			updatedAt                                             time.Time
		)
		if err := rows.Scan(&tenantID, &id, &productID, &name, &source, &target, &version, &rawBody, &status, &publishedAt, &createdAt, &updatedAt); err != nil {
			return nil, nil, err
		}
		if _, ok := tenantSet[tenantID]; !ok {
			continue
		}
		item, err := convertTemplate(tenantID, id, productID, name, source, target, version, status, rawBody, publishedAt, createdAt, updatedAt)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, item)
		templateSet[tenantID+"/"+id] = struct{}{}
	}
	return items, templateSet, rows.Err()
}

func (s *RuntimeConfigSource) loadActiveRoutes(ctx context.Context, tenantSet map[string]struct{}, productSet map[string]struct{}, upstreamSet map[string]struct{}, templateSet map[string]struct{}) ([]route.Route, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT tenant_id::text, id::text, api_product_id::text, inbound_protocol, outbound_protocol,
			COALESCE(host, ''), COALESCE(method, ''), COALESCE(path, ''), upstream_id::text,
			COALESCE(transformation_template_id::text, ''), timeout_ms, required_scopes,
			hmac_enabled, COALESCE(hmac_secret, ''), replay_window_sec, idempotency_enabled, idempotency_ttl_sec
		FROM routes
		WHERE status = 'active' AND deleted_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var routes []route.Route
	for rows.Next() {
		var item route.Route
		if err := rows.Scan(&item.TenantID, &item.ID, &item.APIProductID, &item.InboundProtocol, &item.OutboundProtocol, &item.Host, &item.Method, &item.Path, &item.UpstreamRef, &item.TemplateRef, &item.TimeoutMs, &item.RequiredScopes, &item.HMACEnabled, &item.HMACSecret, &item.ReplayWindowSec, &item.IdempotencyEnabled, &item.IdempotencyTTLSec); err != nil {
			return nil, err
		}
		if _, ok := tenantSet[item.TenantID]; !ok {
			return nil, fmt.Errorf("active route %s belongs to non-active tenant %s", item.ID, item.TenantID)
		}
		if _, ok := productSet[item.TenantID+"/"+item.APIProductID]; !ok {
			return nil, fmt.Errorf("active route %s references non-active api product %s", item.ID, item.APIProductID)
		}
		if _, ok := upstreamSet[item.TenantID+"/"+item.UpstreamRef]; !ok {
			return nil, fmt.Errorf("active route %s references non-active upstream %s", item.ID, item.UpstreamRef)
		}
		if item.TemplateRef != "" {
			if _, ok := templateSet[item.TenantID+"/"+item.TemplateRef]; !ok {
				return nil, fmt.Errorf("active route %s references non-published template %s", item.ID, item.TemplateRef)
			}
		}
		item.Status = route.StatusActive
		item.Method = strings.ToUpper(item.Method)
		routes = append(routes, item)
	}
	return routes, rows.Err()
}

func (s *RuntimeConfigSource) loadActiveCredentials(ctx context.Context, tenantSet map[string]struct{}) ([]auth.APIKeyCredential, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.tenant_id::text, c.id::text, c.consumer_id::text, COALESCE(c.key_prefix, ''), COALESCE(c.secret_hash, ''), c.status,
			COALESCE(t.status, ''), COALESCE(cons.status, ''), c.scopes, c.expires_at
		FROM credentials c
		LEFT JOIN tenants t ON t.id = c.tenant_id
		LEFT JOIN consumers cons ON cons.id = c.consumer_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var credentials []auth.APIKeyCredential
	for rows.Next() {
		item := auth.APIKeyCredential{}
		if err := rows.Scan(&item.TenantID, &item.ID, &item.ConsumerID, &item.KeyPrefix, &item.SecretHash, &item.Status, &item.TenantStatus, &item.ConsumerStatus, &item.Scopes, &item.ExpiresAt); err != nil {
			return nil, err
		}
		if _, ok := tenantSet[item.TenantID]; !ok && item.TenantStatus == "" {
			continue
		}
		credentials = append(credentials, item)
	}
	return credentials, rows.Err()
}

func convertUpstream(tenantID, id, name, protocolName string, rawConfig []byte) (upstream.Upstream, error) {
	item := upstream.Upstream{
		ID:       id,
		TenantID: tenantID,
		Name:     name,
		Protocol: upstream.Protocol(protocolName),
	}
	var cfg map[string]any
	if err := json.Unmarshal(rawConfig, &cfg); err != nil {
		return upstream.Upstream{}, fmt.Errorf("decode upstream %s config: %w", id, err)
	}
	switch protocolName {
	case restprotocol.Name:
		item.BaseURL = stringValue(cfg["baseUrl"])
	case "iso8583":
		item.ISO8583ProfileID = stringValue(cfg["profileId"])
		host := stringValue(cfg["host"])
		port := intValue(cfg["port"])
		if baseURL := stringValue(cfg["baseUrl"]); baseURL != "" {
			item.BaseURL = baseURL
		} else if host != "" && port > 0 {
			item.BaseURL = host + ":" + strconv.Itoa(port)
		}
	case soapxml.Name:
		item.BaseURL = stringValue(cfg["baseUrl"])
		item.SOAPAction = stringValue(cfg["soapAction"])
		item.SOAPOperation = stringValue(cfg["soapOperation"])
		item.SOAPNamespace = stringValue(cfg["soapNamespace"])
		item.SOAPResponsePath = stringValue(cfg["soapResponsePath"])
	default:
		return upstream.Upstream{}, fmt.Errorf("unsupported upstream protocol %q", protocolName)
	}
	if item.BaseURL == "" {
		return upstream.Upstream{}, fmt.Errorf("upstream %s baseUrl is required", id)
	}
	return item, nil
}

func convertTemplate(tenantID, id, productID, name, source, target string, version int, status string, rawBody []byte, publishedAt *time.Time, createdAt, updatedAt time.Time) (transform.Template, error) {
	var body struct {
		Request struct {
			Fields    map[string]string `json:"fields"`
			Sensitive []string          `json:"sensitive"`
		} `json:"request"`
		Response struct {
			Fields    map[string]string `json:"fields"`
			Sensitive []string          `json:"sensitive"`
		} `json:"response"`
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return transform.Template{}, fmt.Errorf("decode template %s body: %w", id, err)
	}
	return transform.Template{
		ID:             id,
		TenantID:       tenantID,
		APIProductID:   productID,
		Name:           name,
		SourceProtocol: source,
		TargetProtocol: target,
		Version:        version,
		Status:         transform.TemplateStatus(status),
		Request: transform.Section{
			Fields:    body.Request.Fields,
			Sensitive: body.Request.Sensitive,
		},
		Response: transform.Section{
			Fields:    body.Response.Fields,
			Sensitive: body.Response.Sensitive,
		},
		PublishedAt: publishedAt,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func stringValue(value any) string {
	out, _ := value.(string)
	return out
}

func intValue(value any) int {
	switch cast := value.(type) {
	case float64:
		return int(cast)
	case int:
		return cast
	case int64:
		return int(cast)
	default:
		return 0
	}
}
