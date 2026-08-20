package multipart

import (
	"context"
	"fmt"
	"strings"
)

type tenantContextKey struct{}

type Tenant struct {
	ID          string
	DisplayName string
	Enabled     bool
	Labels      map[string]string
}

func WithTenant(ctx context.Context, tenant Tenant) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, cloneTenant(tenant))
}

func TenantFromContext(ctx context.Context) (Tenant, bool) {
	if ctx == nil {
		return Tenant{}, false
	}
	tenant, ok := ctx.Value(tenantContextKey{}).(Tenant)
	if !ok || tenant.ID == "" {
		return Tenant{}, false
	}
	return cloneTenant(tenant), true
}

func ValidateTenant(tenant Tenant) error {
	tenant.ID = strings.TrimSpace(tenant.ID)
	if tenant.ID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if strings.ContainsAny(tenant.ID, "/\\ ") {
		return fmt.Errorf("tenant id %q contains a reserved character", tenant.ID)
	}
	if !tenant.Enabled {
		return fmt.Errorf("tenant %s is disabled", tenant.ID)
	}
	return nil
}

func TenantUploadID(tenantID, uploadID string) string {
	return strings.TrimSpace(tenantID) + ":" + strings.TrimSpace(uploadID)
}

func SplitTenantUploadID(value string) (string, string, error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid tenant upload id %q", value)
	}
	return parts[0], parts[1], nil
}

func cloneTenant(tenant Tenant) Tenant {
	out := tenant
	out.Labels = make(map[string]string, len(tenant.Labels))
	for key, value := range tenant.Labels {
		out.Labels[key] = value
	}
	return out
}
