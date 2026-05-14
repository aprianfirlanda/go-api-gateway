package tenant

type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
	StatusDisabled  Status = "disabled"
)

type Tenant struct {
	ID     string
	Name   string
	Status Status
}

type APIProduct struct {
	ID       string
	TenantID string
	Name     string
	Status   Status
}
