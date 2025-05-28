package models

type Metadata struct {
	ExamId        string `json:"examId"`
	Provider      string `json:"provider"`
	Name          string `json:"name"`
	SourceCount   int    `json:"sourceCount"`
	QuestionCount int    `json:"questionCount"`
	ExamCount     int    `json:"examCount"`
	ExamLength    int    `json:"examLength"`
}
