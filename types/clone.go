package types

type CloneRequest struct {
	FileId      string `json:"file_id"`
	DesId       string `json:"des_id"`
	Concurrency int    `json:"concurrency"`
	Size        int64  `json:"size"`
}
