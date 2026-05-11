package runtimeconfig

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"syra-backend/internal/auth"
	"syra-backend/internal/gateway/route"
	"syra-backend/internal/gateway/upstream"
	"syra-backend/internal/protocol/iso8583"
	"syra-backend/internal/transform"
)

type SnapshotSource interface {
	Load(ctx context.Context) (Snapshot, error)
}

type Applier struct {
	Routes      interface{ Replace(...route.Route) }
	Upstreams   interface{ Replace(...upstream.Upstream) }
	Credentials interface {
		Replace(...auth.APIKeyCredential)
	}
	Templates interface{ Replace(...transform.Template) }
	Profiles  interface{ Replace(...iso8583.Profile) }
}

type Manager struct {
	source  SnapshotSource
	applier Applier
	logger  *slog.Logger
	mu      sync.RWMutex
	current Snapshot
	lastErr error
	reloads int64
	rejects int64
}

func NewManager(source SnapshotSource, applier Applier, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{source: source, applier: applier, logger: logger}
}

func (m *Manager) Reload(ctx context.Context) error {
	snapshot, err := m.source.Load(ctx)
	if err != nil {
		m.recordReject(err)
		return err
	}
	if err := snapshot.Validate(); err != nil {
		m.recordReject(err)
		return err
	}
	m.apply(snapshot)
	m.mu.Lock()
	m.current = snapshot
	m.lastErr = nil
	m.reloads++
	m.mu.Unlock()
	m.logger.InfoContext(ctx, "gateway config reloaded", slog.Int64("version", snapshot.Version), slog.String("checksum", snapshot.Checksum))
	return nil
}

func (m *Manager) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 || m.source == nil {
		return
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = m.Reload(ctx)
			}
		}
	}()
}

func (m *Manager) Current() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

func (m *Manager) LastError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastErr
}

func (m *Manager) Stats() (reloads int64, rejects int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reloads, m.rejects
}

func (m *Manager) apply(snapshot Snapshot) {
	if m.applier.Routes != nil {
		m.applier.Routes.Replace(snapshot.Routes...)
	}
	if m.applier.Upstreams != nil {
		m.applier.Upstreams.Replace(snapshot.Upstreams...)
	}
	if m.applier.Credentials != nil {
		m.applier.Credentials.Replace(snapshot.Credentials...)
	}
	if m.applier.Templates != nil {
		m.applier.Templates.Replace(snapshot.Templates...)
	}
	if m.applier.Profiles != nil {
		m.applier.Profiles.Replace(snapshot.Profiles...)
	}
}

func (m *Manager) recordReject(err error) {
	m.mu.Lock()
	m.lastErr = err
	m.rejects++
	m.mu.Unlock()
	m.logger.Warn("gateway config reload rejected", slog.Any("error", err))
}

type StaticSource struct {
	Snapshot Snapshot
	Err      error
}

func (s StaticSource) Load(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if s.Err != nil {
		return Snapshot{}, s.Err
	}
	return s.Snapshot, nil
}
