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
