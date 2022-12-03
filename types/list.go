package types

type ListFilesRequest struct {
	Name     string `json:"name"`
	ParentID string `json:"parent_id"`
	Count    int    `json:"count"`
}
