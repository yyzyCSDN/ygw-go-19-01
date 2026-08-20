package multipart

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
)

type UploadPolicy struct {
	Tenant          string
	Prefix          string
	MaxParts        int
	MaxPartBytes    int
	AllowedRegions  []string
	RequiredStorage string
}

type PolicyRegistry struct {
	mu       sync.RWMutex
	policies map[string][]UploadPolicy
}

func NewPolicyRegistry() *PolicyRegistry {
	return &PolicyRegistry{policies: make(map[string][]UploadPolicy)}
}

func (r *PolicyRegistry) Replace(tenant string, policies []UploadPolicy) error {
	if tenant == "" {
		return fmt.Errorf("tenant is required")
	}
	copyPolicies := make([]UploadPolicy, len(policies))
	for index, policy := range policies {
		if policy.Tenant != "" && policy.Tenant != tenant {
			return fmt.Errorf("policy tenant mismatch")
		}
		if policy.MaxParts <= 0 || policy.MaxPartBytes <= 0 {
			return fmt.Errorf("policy limits must be positive")
		}
		policy.Tenant = tenant
		policy.Prefix = strings.TrimPrefix(path.Clean("/"+policy.Prefix), "/")
		policy.AllowedRegions = append([]string(nil), policy.AllowedRegions...)
		sort.Strings(policy.AllowedRegions)
		copyPolicies[index] = policy
	}
	r.mu.Lock()
	r.policies[tenant] = copyPolicies
	r.mu.Unlock()
	return nil
}

func (r *PolicyRegistry) Resolve(tenant, objectKey string) (UploadPolicy, bool) {
	r.mu.RLock()
	policies := clonePolicies(r.policies[tenant])
	r.mu.RUnlock()
	var selected UploadPolicy
	matched := false
	for _, policy := range policies {
		if policy.Prefix != "" && !strings.HasPrefix(objectKey, policy.Prefix) {
			continue
		}
		if !matched || len(policy.Prefix) > len(selected.Prefix) {
			selected, matched = policy, true
		}
	}
	return clonePolicy(selected), matched
}

func (r *PolicyRegistry) List(tenant string) []UploadPolicy {
	r.mu.RLock()
	result := clonePolicies(r.policies[tenant])
	r.mu.RUnlock()
	return result
}

func (p UploadPolicy) ValidatePart(part Part) error {
	if len(part.Data) > p.MaxPartBytes {
		return fmt.Errorf("part %d exceeds policy limit", part.Number)
	}
	if p.RequiredStorage != "" && (part.Metadata == nil || part.Metadata.StorageClass != p.RequiredStorage) {
		return fmt.Errorf("part %d storage class is not allowed", part.Number)
	}
	if len(p.AllowedRegions) == 0 {
		return nil
	}
	region := ""
	if part.Metadata != nil {
		region = part.Metadata.Region
	}
	index := sort.SearchStrings(p.AllowedRegions, region)
	if index >= len(p.AllowedRegions) || p.AllowedRegions[index] != region {
		return fmt.Errorf("part %d region %q is not allowed", part.Number, region)
	}
	return nil
}

func clonePolicies(values []UploadPolicy) []UploadPolicy {
	result := make([]UploadPolicy, len(values))
	for index, value := range values {
		result[index] = clonePolicy(value)
	}
	return result
}

func clonePolicy(value UploadPolicy) UploadPolicy {
	value.AllowedRegions = append([]string(nil), value.AllowedRegions...)
	return value
}
