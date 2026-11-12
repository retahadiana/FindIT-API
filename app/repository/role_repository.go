package repository

import (
	"context"
	"findit-api/helper"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RoleRepository interface {
	LoadPermissions(ctx context.Context) (*helper.PermissionSet, error)
}

type rolePostgresRepository struct{ pool *pgxpool.Pool }

func NewRoleRepository(pool *pgxpool.Pool) RoleRepository {
	return &rolePostgresRepository{pool: pool}
}

func (r *rolePostgresRepository) LoadPermissions(ctx context.Context) (*helper.PermissionSet, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.name, rp.permission_name
		FROM roles r
		LEFT JOIN role_permissions rp ON rp.role_name = r.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	mapping := make(map[string][]string)
	for rows.Next() {
		var role string
		var perm *string
		if err := rows.Scan(&role, &perm); err != nil {
			return nil, err
		}
		if _, ok := mapping[role]; !ok {
			mapping[role] = []string{}
		}
		if perm != nil {
			mapping[role] = append(mapping[role], *perm)
		}
	}
	return helper.NewPermissionSet(mapping), rows.Err()
}
