package helper

import "sort"

type PermissionSet struct {
	roles map[string]map[string]bool
}

func NewPermissionSet(mapping map[string][]string) *PermissionSet {
	roles := make(map[string]map[string]bool, len(mapping))
	for role, perms := range mapping {
		set := make(map[string]bool, len(perms))
		for _, p := range perms {
			set[p] = true
		}
		roles[role] = set
	}
	return &PermissionSet{roles: roles}
}

func DefaultPermissions() *PermissionSet {
	return NewPermissionSet(map[string][]string{
		"admin": {"user:list", "user:read:any", "user:update:any", "user:delete", "role:assign", "category:manage", "queue:monitor"},
		"staff": {"user:list", "user:read:any"},
		"user":  {},
	})
}

func (s *PermissionSet) Has(role, perm string) bool {
	if s == nil {
		return false
	}
	return s.roles[role][perm]
}

func (s *PermissionSet) ListForRole(role string) []string {
	if s == nil || len(s.roles[role]) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(s.roles[role]))
	for p := range s.roles[role] {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
