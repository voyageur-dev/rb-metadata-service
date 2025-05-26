package models

type GetCountResponsePayload struct {
	Body GetCountResponsePayloadBody `json:"body"`
}

type GetCountResponsePayloadBody struct {
	Count map[string]int `json:"count"`
}
