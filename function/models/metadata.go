package models

type Metadata struct {
	ExamId        string `json:"examId"`
	ProviderId    string `json:"providerId"`
	ExamName      string `json:"examName"`
	SourceCount   int    `json:"sourceCount"`
	QuestionCount int    `json:"questionCount"`
	ExamCount     int    `json:"examCount"`
	ExamLength    int    `json:"examLength"`
}
