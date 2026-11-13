package service

import (
	"strings"
	"findit-api/app/model"
)

func IsEmptyPatch(req model.PatchUserRequest) bool {
	return req.FullName == nil && req.Phone == nil
}

func ApplyPatch(existing model.User, req model.PatchUserRequest) model.User {
	if req.FullName != nil {
		existing.FullName = strings.TrimSpace(*req.FullName)
	}
	if req.Phone != nil {
		existing.Phone = strings.TrimSpace(*req.Phone)
	}
	return existing
}
