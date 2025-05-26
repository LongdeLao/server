package config

// Database configuration
const (
	DBHost     = "69.62.73.139"
	DBName     = "HSANNU"
	DBUser     = "postgres"
	DBPassword = "2008"
	DBPort     = "5433"
)

// APNs configuration
const (
	AuthKeyPath = "key.p8"
	AuthKeyID   = "BK88TAV8F8"
	TeamID      = "CNSN2FZNRR"
	APNSTopic   = "com.leo.hsannu"
)

// SMTP Email configuration
const (
	SMTPHost     = "smtp.hostinger.com"
	SMTPPort     = "587"
	SMTPUsername = "support@hsannu.com" // Replace with actual email
	SMTPPassword = "Hsannu_123"         // Replace with actual app password
	SMTPSender   = "HSANNU Support <support@hsannu.com>"
)

// Server configuration
const ServerPort = "2000"

// AI Assistant configuration
const (
	AIAPIKey          = "sk-96744d1c86d149dcb3f906b8fcbcd210"
	AIBaseURL         = "https://api.deepseek.com/chat/completions"
	AIMaxInputTokens  = 4000
	AIMaxOutputTokens = 2000
	AITemperature     = 0.7
	AITopP            = 0.9
)
