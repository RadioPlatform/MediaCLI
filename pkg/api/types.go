package api

type Station struct {
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	IsActive *bool  `json:"is_active,omitempty"`
}

// Folder is a media library folder path on a station.
type Folder struct {
	Name string `json:"name"`
}

type MediaItem struct {
	UUID             string `json:"uuid,omitempty"`
	Filename         string `json:"filename,omitempty"`
	OriginalFilename string `json:"original_filename,omitempty"`
	Title            string `json:"title,omitempty"`
	Folder           string `json:"folder,omitempty"`
	SizeBytes        int64  `json:"size_bytes,omitempty"`
	DurationSeconds  int    `json:"duration_seconds,omitempty"`
	IsJingle         bool   `json:"is_jingle"`

	// Legacy fields kept for older fixtures/tests.
	ID       int   `json:"id,omitempty"`
	Size     int64 `json:"size,omitempty"`
	Duration int   `json:"duration,omitempty"`
}

func (m MediaItem) DisplayFilename() string {
	if m.OriginalFilename != "" {
		return m.OriginalFilename
	}
	if m.Filename != "" {
		return m.Filename
	}
	return ""
}

func (m MediaItem) DisplaySize() int64 {
	if m.SizeBytes > 0 {
		return m.SizeBytes
	}
	return m.Size
}

func (m MediaItem) DisplayDuration() int {
	if m.DurationSeconds > 0 {
		return m.DurationSeconds
	}
	return m.Duration
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
	Data struct {
		Folder string `json:"folder"`
	} `json:"data"`
}

type UploadResult struct {
	Media   *MediaItem `json:"media,omitempty"`
	Success bool       `json:"success"`
	Error   string     `json:"error,omitempty"`
}

type UploadProgress struct {
	UploadID       string `json:"upload_id"`
	ChunkIndex     int    `json:"chunk_index"`
	ChunksReceived int    `json:"chunks_received"`
	TotalChunks    int    `json:"total_chunks"`
	BytesReceived  int64  `json:"bytes_received"`
	TotalBytes     int64  `json:"total_size"`
	Complete       bool   `json:"complete"`
}

type UploadProgressFunc func(UploadProgress)

type chunkUploadResponse struct {
	Data struct {
		UploadProgress
		Track *MediaItem `json:"track,omitempty"`
	} `json:"data"`
}
