package model

// WebResponse adalah amplop baku untuk seluruh respons API.
type WebResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Meta    any    `json:"meta,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

// Meta menyimpan informasi paginasi berbasis halaman.
type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// ListQuery menyimpan parameter query string untuk daftar terpaginasi.
type ListQuery struct {
	Page   int
	Limit  int
	Search string
	Sort   string
	Order  string
}

// Offset menghitung jumlah baris yang dilewati pada halaman saat ini.
func (q ListQuery) Offset() int {
	return (q.Page - 1) * q.Limit
}

// ErrorResponse adalah amplop baku untuk respons kegagalan HTTP.
type ErrorResponse struct {
	Success   bool              `json:"success"`
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}


type Cursor struct {
	CreatedAt int64 `json:"created_at"`
	ID        int   `json:"id"`
}

type CursorMeta struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

type CursorQuery struct {
	After  *Cursor
	Limit  int
	Search string
}
