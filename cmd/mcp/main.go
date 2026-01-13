package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	ai "go-bank-api/ai"
	config "go-bank-api/config"
	database "go-bank-api/database"
	"go-bank-api/models"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	log.SetOutput(os.Stderr)

	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	if _, err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := database.ConnectDB(); err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}

	if err := ai.InitVectorService(); err != nil {
		log.Printf("Warning: Vector service init failed: %v. AI features might be limited.", err)
	}

	s := server.NewMCPServer(
		"Go Bank API MCP",
		"1.0.0",
		server.WithLogging(),
	)

	tool := mcp.NewTool("query_bank_data",
		mcp.WithDescription("Ask questions about bank data in natural language. Capable of checking balances, transactions, and customer details. Always use this tool for any data retrieval request."),
		mcp.WithString("prompt",
			mcp.Description("The natural language question or instruction from the user (e.g., 'Check Budi's balance', 'List recent transactions')."),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("Arguments must be a JSON object"), nil
		}

		prompt, ok := args["prompt"].(string)
		if !ok {
			return mcp.NewToolResultError("Prompt argument must be a string"), nil
		}

		log.Printf("[MCP] Handling Query: %s", prompt)

		aiResp, err := ai.GetSQL(prompt)
		if err != nil {
			if appErr, ok := err.(*models.AppError); ok {
				return mcp.NewToolResultText(fmt.Sprintf("Error (%s): %s", appErr.Code, appErr.Message)), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("AI Generation Failed: %v", err)), nil
		}

		if aiResp.IsAmbiguous {
			msg := "Pertanyaan ambigu. Mungkin maksud Anda:\n"
			for _, sugg := range aiResp.Suggestions {
				msg += fmt.Sprintf("- %s\n", sugg)
			}
			return mcp.NewToolResultText(msg), nil
		}

		log.Printf("[MCP] Executing SQL: %s", aiResp.SQL)
		data, execErr := ai.ExecuteDynamicQuery(aiResp.SQL, nil)
		if execErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Query Execution Failed: %v\nSQL: %s", execErr, aiResp.SQL)), nil
		}

		jsonBytes, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return mcp.NewToolResultError("Failed to serialize response data"), nil
		}

		responseText := fmt.Sprintf("✅ **Success**\n\n**Generated SQL**:\n```sql\n%s\n```\n\n**Result**:\n```json\n%s\n```", aiResp.SQL, string(jsonBytes))

		return mcp.NewToolResultText(responseText), nil
	})

	log.Println("MCP Server running on Stdio...")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
