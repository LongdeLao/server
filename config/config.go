package config

// Database configuration
const (
	DBHost     = "69.62.73.139"
	DBName     = "HSANNU"
	DBUser     = "postgres"
	DBPassword = "2008"
	DBPort     = "8080"
)

// APNs configuration
const (
	AuthKeyPath = "key.p8"
	AuthKeyID   = "BK88TAV8F8"
	TeamID      = "CNSN2FZNRR"
	APNSTopic   = "com.leo.hsannu"
)

// APNSEnvironment is set at runtime (development or production)
var APNSEnvironment = "development"

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
