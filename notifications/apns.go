package notifications

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"server/config"
	"time"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"
)

var (
	client      *apns2.Client
	initialized bool = false
)

// InitAPNS initializes the APNS client
func InitAPNS() error {
	if initialized {
		return nil
	}

	// Read the private key
	bytes, err := ioutil.ReadFile(config.AuthKeyPath)
	if err != nil {
		return fmt.Errorf("unable to read APNs key file: %v", err)
	}

	// Create a new token using the P8 file
	authKey, err := token.AuthKeyFromBytes(bytes)
	if err != nil {
		return fmt.Errorf("unable to load APNs key: %v", err)
	}

	// Create the token provider
	token := &token.Token{
		AuthKey: authKey,
		KeyID:   config.AuthKeyID,
		TeamID:  config.TeamID,
	}

	// Initialize the client
	client = apns2.NewTokenClient(token).Development()
	initialized = true
	log.Println("APNs client initialized successfully")
	return nil
}

// SendMessageNotification sends a push notification about a new message
func SendMessageNotification(deviceToken string, conversationID int, senderName string, messageContent string) error {
	if !initialized {
		if err := InitAPNS(); err != nil {
			return err
		}
	}

	// Validate device token
	if deviceToken == "" {
		return fmt.Errorf("empty device token")
	}

	// Create the notification payload
	p := payload.NewPayload()
	p.AlertTitle("New Message from " + senderName)
	p.AlertBody(messageContent)
	p.Badge(1)
	p.Sound("default")
	p.Category("MESSAGE")

	// Add custom data for deep linking
	p.Custom("conversationID", conversationID)
	p.Custom("messageType", "chat")

	// Create the notification
	notification := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       config.APNSTopic,
		Payload:     p,
		Priority:    apns2.PriorityHigh,
		Expiration:  time.Now().Add(24 * time.Hour),
	}

	// Send the notification
	res, err := client.Push(notification)
	if err != nil {
		return fmt.Errorf("failed to send APNs notification: %v", err)
	}

	// Log the result
	log.Printf("APNs Notification sent to %s: %v", deviceToken, res)

	if res.StatusCode != 200 {
		return fmt.Errorf("APNs notification failed with status %d: %s", res.StatusCode, res.Reason)
	}

	return nil
}

// SendRefreshNotification sends a silent notification to refresh app content
func SendRefreshNotification(deviceToken string, refreshType string) error {
	if !initialized {
		if err := InitAPNS(); err != nil {
			return err
		}
	}

	// Validate device token
	if deviceToken == "" {
		return fmt.Errorf("empty device token")
	}

	// Create the silent notification payload
	p := payload.NewPayload()
	p.ContentAvailable()
	p.Custom("refresh", refreshType) // could be "messages", "events", etc.

	// Create the notification
	notification := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       config.APNSTopic,
		Payload:     p,
		Priority:    apns2.PriorityLow, // Low priority for silent notifications
		Expiration:  time.Now().Add(1 * time.Hour),
	}

	// Send the notification
	res, err := client.Push(notification)
	if err != nil {
		return fmt.Errorf("failed to send silent notification: %v", err)
	}

	// Log the result
	log.Printf("Silent notification sent to %s: %v", deviceToken, res)

	if res.StatusCode != 200 {
		return fmt.Errorf("silent notification failed with status %d: %s", res.StatusCode, res.Reason)
	}

	return nil
}

// SendLiveActivityUpdate sends a push notification to update a Live Activity
func SendLiveActivityUpdate(activityToken string, status string, responseTime time.Time, respondedBy string) error {
	if !initialized {
		if err := InitAPNS(); err != nil {
			return err
		}
	}

	// Validate activity token
	if activityToken == "" {
		return fmt.Errorf("empty activity token")
	}

	// Format response time to ISO8601
	timeString := responseTime.Format(time.RFC3339)

	// Create a custom payload for Live Activity
	type ContentState struct {
		Status       string  `json:"status"`
		ResponseTime *string `json:"responseTime,omitempty"`
		RespondedBy  *string `json:"respondedBy,omitempty"`
	}

	// Prepare content state based on status
	var contentState ContentState
	if status == "pending" {
		contentState = ContentState{
			Status:       status,
			ResponseTime: nil,
			RespondedBy:  nil,
		}
	} else {
		contentState = ContentState{
			Status:       status,
			ResponseTime: &timeString,
			RespondedBy:  &respondedBy,
		}
	}

	// Create the Live Activity payload
	liveActivityPayload := map[string]interface{}{
		"aps": map[string]interface{}{
			"event":         "update",
			"timestamp":     time.Now().Unix(),
			"content-state": contentState,
		},
	}

	// Convert to JSON
	payloadBytes, err := json.Marshal(liveActivityPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal Live Activity payload: %v", err)
	}

	// Create the notification
	notification := &apns2.Notification{
		DeviceToken: activityToken,
		Topic:       fmt.Sprintf("%s.push-type.liveactivity", config.APNSTopic),
		Payload:     payloadBytes,
		Priority:    apns2.PriorityHigh,
		PushType:    apns2.PushTypeLiveActivity,
	}

	// Send the notification
	res, err := client.Push(notification)
	if err != nil {
		return fmt.Errorf("failed to send Live Activity update: %v", err)
	}

	// Log the result
	log.Printf("Live Activity update sent to token %s: %v", activityToken, res)

	if res.StatusCode != 200 {
		return fmt.Errorf("Live Activity update failed with status %d: %s", res.StatusCode, res.Reason)
	}

	return nil
}

// SendAPNsNotification sends a push notification with a custom JSON payload
func SendAPNsNotification(deviceToken string, topic string, jsonPayload string, isLiveActivity bool) (string, error) {
	if !initialized {
		if err := InitAPNS(); err != nil {
			return "", err
		}
	}

	// Validate device token
	if deviceToken == "" {
		return "", fmt.Errorf("empty device token")
	}

	// Create the notification
	notification := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       topic,
		Payload:     []byte(jsonPayload),
		Priority:    apns2.PriorityHigh,
		Expiration:  time.Now().Add(24 * time.Hour),
	}

	// Set push type if it's a Live Activity notification
	if isLiveActivity {
		notification.PushType = apns2.PushTypeLiveActivity
	}

	// Send the notification
	res, err := client.Push(notification)
	if err != nil {
		return "", fmt.Errorf("failed to send APNs notification: %v", err)
	}

	// Log the result
	log.Printf("APNs Notification sent to %s: %v", deviceToken, res)

	if res.StatusCode != 200 {
		return "", fmt.Errorf("APNs notification failed with status %d: %s", res.StatusCode, res.Reason)
	}

	return fmt.Sprintf("Success - APNs notification sent with status: %d", res.StatusCode), nil
}

// SendLeaveRequestStatusUpdate sends a push notification specifically for leave request status changes
func SendLeaveRequestStatusUpdate(deviceToken string, activityId string, status string, staffName string) (string, error) {
	if !initialized {
		if err := InitAPNS(); err != nil {
			return "", err
		}
	}

	// Validate inputs
	if deviceToken == "" {
		return "", fmt.Errorf("empty device token")
	}

	if activityId == "" {
		return "", fmt.Errorf("empty activity ID")
	}

	// Get current time for the response time
	responseTime := time.Now()
	
	// Create content state with response details
	var contentState map[string]interface{}
	
	// Only include responseTime and respondedBy for non-pending statuses
	if status == "pending" {
		contentState = map[string]interface{}{
			"status": status,
			// No responseTime or respondedBy for pending status
		}
	} else {
		timeString := responseTime.Format(time.RFC3339)
		contentState = map[string]interface{}{
			"status":       status,
			"responseTime": timeString,
			"respondedBy":  staffName,
		}
	}

	// Build the complete payload - EXACTLY matching the shell script format
	payload := map[string]interface{}{
		"aps": map[string]interface{}{
			"event":         "update",
			"timestamp":     time.Now().Unix(),
			"content-state": contentState,
		},
		"activity-id": activityId,
	}

	// Convert to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Live Activity payload: %v", err)
	}

	// Log the outgoing payload for debugging
	log.Printf("📱 Sending Live Activity status update: %s", string(payloadBytes))

	// The bundle ID for Live Activities needs .push-type.liveactivity appended
	bundleID := fmt.Sprintf("%s.push-type.liveactivity", config.APNSTopic)
	
	// Add more detailed logging
	log.Printf("📲 DETAILED APNS DATA:")
	log.Printf("Token: %s", deviceToken)
	log.Printf("Bundle ID: %s", bundleID)
	log.Printf("Push Type: liveactivity")
	log.Printf("Activity ID: %s", activityId)
	log.Printf("Status: %s", status)

	// Send the notification using our existing method
	return SendAPNsNotification(deviceToken, bundleID, string(payloadBytes), true)
}
