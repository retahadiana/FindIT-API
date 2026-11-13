package service

import (
	"errors"
	"testing"

	"findit-api/app/model"
	"findit-api/helper"
)

func TestCanAccessUser(t *testing.T) {
	perms := helper.DefaultPermissions()
	tests := []struct {
		name   string
		actor  model.AuthUser
		target int
		want   bool
	}{
		{"pemilik akun sendiri", model.AuthUser{UserID: 5, Role: "user"}, 5, true},
		{"user lain ditolak", model.AuthUser{UserID: 5, Role: "user"}, 9, false},
		{"admin boleh akses siapa saja", model.AuthUser{UserID: 1, Role: "admin"}, 9, true},
		{"staff punya user:read:any", model.AuthUser{UserID: 2, Role: "staff"}, 9, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanAccessUser(tt.actor, tt.target, perms, "user:read:any"); got != tt.want {
				t.Fatalf("CanAccessUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateAssignRole_WhiteBoxErrors(t *testing.T) {
	actor := model.AuthUser{UserID: 1, Role: "admin"}
	var appErr *helper.AppError

	errSelf := ValidateAssignRole(actor, 1, "staff")
	if !errors.As(errSelf, &appErr) || appErr.Status != 403 {
		t.Fatalf("self role change status = %v, want 403", errSelf)
	}

	errInvalid := ValidateAssignRole(actor, 2, "superadmin")
	if !errors.As(errInvalid, &appErr) || appErr.Status != 400 {
		t.Fatalf("invalid role status = %v, want 400", errInvalid)
	}

	if err := ValidateAssignRole(actor, 2, "staff"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserPatchRules(t *testing.T) {
	if !IsEmptyPatch(model.PatchUserRequest{}) {
		t.Fatal("patch kosong harus terdeteksi")
	}
	name := "  Lifia Baru  "
	patched := ApplyPatch(model.User{FullName: "Lama", Phone: "081"}, model.PatchUserRequest{FullName: &name})
	if patched.FullName != "Lifia Baru" || patched.Phone != "081" {
		t.Fatalf("ApplyPatch tidak sesuai: %+v", patched)
	}
}
