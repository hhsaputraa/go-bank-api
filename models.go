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
	SQL        string
	Vector     []float32
	PromptAsli string
	IsCached   bool
}
