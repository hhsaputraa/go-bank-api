package constants

// HTTP Status Messages
const (
	StatusOK      = "OK"
	StatusSuccess = "SUCCESS"
	StatusFailed  = "FAILED"
)

// Error Codes
const (
	ErrCodeMethodNotAllowed   = "METHOD_NOT_ALLOWED"
	ErrCodeInvalidJSON        = "INVALID_JSON"
	ErrCodeEmptyPrompt        = "EMPTY_PROMPT"
	ErrCodeInvalidPrompt      = "INVALID_PROMPT"
	ErrCodeDangerousIntent    = "DANGEROUS_INTENT"
	ErrCodeChitChat           = "CHIT_CHAT"
	ErrCodeAmbiguous          = "AMBIGUOUS"
	ErrCodeEmptySQL           = "EMPTY_SQL"
	ErrCodeQueryFailed        = "QUERY_EXECUTION_FAILED"
	ErrCodeAIGenerationFailed = "AI_GENERATION_FAILED"
	ErrCodeUnauthorized       = "UNAUTHORIZED"
	ErrCodeInvalidToken       = "INVALID_TOKEN"
	ErrCodeSessionExpired     = "SESSION_EXPIRED"
	ErrCodeSessionError       = "SESSION_ERROR"
	ErrCodeInternalError      = "INTERNAL_ERROR"
	ErrCodeBadFormat          = "BAD_FORMAT"
	ErrCodeEnhanceFailed      = "ENHANCE_FAILED"
	ErrCodeDecryptFail        = "DECRYPT_FAIL"
	ErrCodeLoginFailed        = "LOGIN_FAILED"
	ErrCodeAiRefusal          = "AI_REFUSAL"
)

// Intent Types
const (
	IntentUnknown    = "UNKNOWN"
	IntentSQL        = "SQL"
	IntentChat       = "CHAT"
	IntentOffTopic   = "OFF_TOPIC"
	IntentAmbiguous  = "AMBIGUOUS"
	IntentAttack     = "ATTACK"
	IntentSQLAttempt = "SQL_ATTEMPT"
)

// Cache Thresholds
const (
	// CacheHardHitThreshold is the minimum similarity score for a direct cache hit
	// Value from config: CACHE_SIMILARITY_THRESHOLD (default 0.99)
	CacheHardHitThreshold = 0.99

	// CacheSoftHitThreshold is the minimum similarity score for using cache as context
	CacheSoftHitThreshold = 0.80
)

// RAG Score Thresholds
const (
	// RAGMinimumScore is the minimum similarity score to use RAG context
	RAGMinimumScore = 0.45

	// DDLMinimumScore is the minimum similarity score to use DDL context
	DDLMinimumScore = 0.30
)

// User Roles
const (
	// AdminRoleValue is the database value that indicates admin role
	// This replaces the magic number "7" in the codebase
	AdminRoleValue = 7

	// RegularUserRole is the database value for regular users
	RegularUserRole = 0
)

// Database Values
const (
	// ActiveUserStatus indicates an active user account
	ActiveUserStatus = 1

	// InactiveUserStatus indicates an inactive user account
	InactiveUserStatus = 0

	// Boolean constants for database (Oracle NUMBER(1))
	TrueValue  = 1
	FalseValue = 0

	// Account Status
	AccountStatusPerfect        = 0
	AccountStatusPendingSetup   = 1
	AccountStatusForgotPassword = 2
	AccountStatusBlocked        = 3
)

// LLM Models
const (
	// GroqModelFast is the fast Groq model for quick responses
	GroqModelFast = "llama-3.1-8b-instant"

	// GroqModelDefault is the default model from config
	GroqModelDefault = "qwen/qwen3.6-27b"

	// Available Models
	ModelQwen32B    = "qwen/qwen3.6-27b"
	ModelGPTOss120B = "openai/gpt-oss-120b" // Note: Check provider availability
	ModelGPTOss20B  = "openai/gpt-oss-20b"  // Note: Check provider availability
)

// Qdrant Categories
const (
	CategorySQL = "sql"
	CategoryDDL = "ddl"
)

// Context Keys for request context
type ContextKey string

const (
	ContextKeyUserID   ContextKey = "user_id"
	ContextKeyUsername ContextKey = "username"
	ContextKeyIsAdmin  ContextKey = "is_admin"
)
