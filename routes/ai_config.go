package routes

import (
	"net/http"
	"server/config"

	"github.com/gin-gonic/gin"
)

/**
 * AIConfigResponse represents the AI configuration response.
 */
type AIConfigResponse struct {
	APIKey          string  `json:"api_key"`
	BaseURL         string  `json:"base_url"`
	MaxInputTokens  int     `json:"max_input_tokens"`
	MaxOutputTokens int     `json:"max_output_tokens"`
	Temperature     float64 `json:"temperature"`
	TopP            float64 `json:"top_p"`
}

/**
 * RegisterAIConfigRoutes registers AI configuration routes.
 *
 * Endpoints:
 * 1. GET /ai/config
 *    - Returns AI configuration settings
 */
func RegisterAIConfigRoutes(router gin.IRouter) {
	router.GET("/ai/config", getAIConfig)
}

/**
 * getAIConfig returns the AI configuration.
 *
 * Endpoint: GET /ai/config
 *
 * Returns:
 *   - 200 OK: AI configuration
 *     {
 *       "api_key": string,
 *       "base_url": string,
 *       "max_input_tokens": number,
 *       "max_output_tokens": number,
 *       "temperature": number,
 *       "top_p": number
 *     }
 */
func getAIConfig(c *gin.Context) {
	response := AIConfigResponse{
		APIKey:          config.AIAPIKey,
		BaseURL:         config.AIBaseURL,
		MaxInputTokens:  config.AIMaxInputTokens,
		MaxOutputTokens: config.AIMaxOutputTokens,
		Temperature:     config.AITemperature,
		TopP:            config.AITopP,
	}

	c.JSON(http.StatusOK, response)
}
