package ai

import (
	"testing"
)

func TestStripThinkTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Text without think tags",
			input:    "result = df.head(10)",
			expected: "result = df.head(10)",
		},
		{
			name:     "Text with think tags",
			input:    "<think>Analisis: mencari top 10 nasabah</think>result = df.head(10)",
			expected: "result = df.head(10)",
		},
		{
			name:     "Multiline think block",
			input:    "<think>\nLangkah 1: Cek kolom\nLangkah 2: Filter\n</think>\nresult = df[df['SALDO'] > 1000000]",
			expected: "result = df[df['SALDO'] > 1000000]",
		},
		{
			name:     "Unclosed think tag",
			input:    "<think>unfinished thought...",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripThinkTags(tt.input)
			if result != tt.expected {
				t.Errorf("StripThinkTags(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractPythonCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Raw code line",
			input:    "result = df['NILAI_BUNGA'].sum()",
			expected: "result = df['NILAI_BUNGA'].sum()",
		},
		{
			name:     "Markdown python codeblock",
			input:    "```python\nresult = df['NILAI_BUNGA'].sum()\n```",
			expected: "result = df['NILAI_BUNGA'].sum()",
		},
		{
			name:     "Markdown py codeblock",
			input:    "```py\nresult = df.groupby('KODE_KANTOR')['NILAI_BUNGA'].sum()\n```",
			expected: "result = df.groupby('KODE_KANTOR')['NILAI_BUNGA'].sum()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPythonCode(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractPythonCode(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCleanPandasCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Standard single line",
			input:    "result = df.nlargest(1, 'NILAI_BUNGA')",
			expected: "result = df.nlargest(1, 'NILAI_BUNGA')",
		},
		{
			name:     "Think tags and code block",
			input:    "<think>Mencari nilai bunga tertinggi</think>\n```python\n# Komentar\nresult = df.nlargest(1, 'NILAI_BUNGA')\n```",
			expected: "result = df.nlargest(1, 'NILAI_BUNGA')",
		},
		{
			name:     "No result prefix fallback",
			input:    "df['SALDO'].sum()",
			expected: "df['SALDO'].sum()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanPandasCode(tt.input)
			if result != tt.expected {
				t.Errorf("CleanPandasCode(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}
