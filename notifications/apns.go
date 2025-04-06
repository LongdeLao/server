package notifications

import (
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
