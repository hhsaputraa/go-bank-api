package main

type QueryRequest struct {
	Laporan string `json:"laporan"`
	Target  string `json:"target"`
	ID      string `json:"id"`
	Periode string `json:"periode"`
}

type PromptRequest struct {
	Prompt string `json:"prompt"`
}

type FeedbackRequest struct {
	PromptAsli string `json:"prompt_asli"`
	SqlKoreksi string `json:"sql_koreksi"`
}

type AISqlResponse struct {
	SQL         string
	Vector      []float32
	PromptAsli  string
	IsCached    bool
	IsAmbiguous bool
	Suggestions []string
}

type SqlExample struct {
	FullContent string
	PromptOnly  string
}

type AppError struct {
	Code    string `json:"code"`    // untuk mapping di frontend
	Message string `json:"message"` // untuk user
	Detail  string `json:"detail,omitempty"`
}

type QueryResponse struct {
	Status      string      `json:"status"`
	Message     string      `json:"message,omitempty"`
	Data        interface{} `json:"data,omitempty"`
	Suggestions []string    `json:"suggestions,omitempty"`
	ErrorCode   string      `json:"error_code,omitempty"`
	ErrorDetail string      `json:"error_detail,omitempty"`
}
