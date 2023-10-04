package routes

type PaginatedRequest struct {
	Offset int `json:"offset" validate:"gte=0"`
	Limit  int `json:"limit" validate:"gte=0,lte=100"`
}

type TxIDsResponse struct {
	TxIDs []string `json:"txIds"`
}
