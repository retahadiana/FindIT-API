package helper

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"findit-api/app/model"
)

var ErrInvalidCursor = errors.New("cursor tidak valid")

func EncodeCursor(createdAt time.Time, id int) string {
	raw := fmt.Sprintf("%d|%d", createdAt.UTC().UnixNano(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(token string) (*model.Cursor, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return nil, ErrInvalidCursor
	}
	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || ts <= 0 {
		return nil, ErrInvalidCursor
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil || id < 1 {
		return nil, ErrInvalidCursor
	}
	return &model.Cursor{CreatedAt: ts, ID: id}, nil
}
