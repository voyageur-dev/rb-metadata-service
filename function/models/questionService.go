package models

type GetCountResponse struct {
	Payload GetCountResponsePayload `json:"payload"`
}

type GetCountResponsePayload struct {
	Body GetCountResponsePayloadBody `json:"body"`
}

type GetCountResponsePayloadBody struct {
	Count map[string]int `json:"count"`
}
