package api

type Station struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type Folder struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	MediaCount int    `json:"media_count"`
}

type MediaItem struct {
	ID       int    `json:"id"`
	Filename string `json:"filename"`
	Title    string `json:"title,omitempty"`
	Folder   string `json:"folder,omitempty"`
	Size     int64  `json:"size"`
	Duration int    `json:"duration,omitempty"`
	IsJingle bool   `json:"is_jingle"`
}

type PaginationMeta struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
}

type Links struct {
	First string `json:"first,omitempty"`
	Last  string `json:"last,omitempty"`
	Prev  string `json:"prev,omitempty"`
	Next  string `json:"next,omitempty"`
}

type StationsResponse struct {
	Data []Station `json:"data"`
}

type FoldersResponse struct {
	Data []Folder `json:"data"`
}

type MediaResponse struct {
	Data  []MediaItem    `json:"data"`
	Meta  PaginationMeta `json:"meta"`
	Links Links          `json:"links,omitempty"`
}

type CreateFolderRequest struct {
	Name string `json:"name"`
}

type CreateFolderResponse struct {
	Data Folder `json:"data"`
}

type UploadResult struct {
	Media   *MediaItem `json:"media,omitempty"`
	Success bool       `json:"success"`
	Error   string     `json:"error,omitempty"`
}
