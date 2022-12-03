package types

type DownloadRequest struct {
	FileId      string `json:"file_id"`
	LocalDir    string `json:"local_dir"`
	Size        int64  `json:"size"`
	Concurrency int    `json:"concurrency"`
}
