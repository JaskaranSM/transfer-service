package types

type UploadRequest struct {
	Path        string `json:"path"`
	ParentId    string `json:"parent_id"`
	Concurrency int    `json:"concurrency"`
	Size        int64  `json:"size"`
}
