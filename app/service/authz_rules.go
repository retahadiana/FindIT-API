package service

import (
	"findit-api/app/model"
	"findit-api/helper"
)

func CanAccessUser(actor model.AuthUser, targetID int, perms *helper.PermissionSet, anyPerm string) bool {
	return actor.UserID == targetID || perms.Has(actor.Role, anyPerm)
}

func ValidateAssignRole(actor model.AuthUser, targetID int, newRole string) error {
	if actor.UserID == targetID {
		return helper.Forbidden("tidak boleh mengubah role milik diri sendiri")
	}
	if newRole != "admin" && newRole != "staff" && newRole != "user" {
		return helper.BadRequest("role harus admin, staff, atau user")
	}
	return nil
}
