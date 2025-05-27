#!/bin/bash

# Set default values
DEFAULT_HOUR=23
DEFAULT_MINUTE=40
DEFAULT_ENABLED=true

echo "=========================================="
echo "🔧 Auto-Mark Settings Updater"
echo "=========================================="

# Ask user for hour or use default
read -p "Enter UTC hour (0-23) [$DEFAULT_HOUR]: " HOUR
HOUR=${HOUR:-$DEFAULT_HOUR}

# Ask user for minute or use default
read -p "Enter UTC minute (0-59) [$DEFAULT_MINUTE]: " MINUTE
MINUTE=${MINUTE:-$DEFAULT_MINUTE}

# Ask user for enabled status or use default
read -p "Enable auto-marking? (true/false) [$DEFAULT_ENABLED]: " ENABLED
ENABLED=${ENABLED:-$DEFAULT_ENABLED}

# Calculate Shanghai time (UTC+8)
SHANGHAI_HOUR=$(( (HOUR + 8) % 24 ))

echo ""
echo "You are about to set auto-marking to:"
echo "- UTC Time:       $HOUR:$MINUTE"
echo "- Shanghai Time:  $SHANGHAI_HOUR:$MINUTE"
echo "- Enabled:        $ENABLED"
echo ""

read -p "Proceed? (y/n): " CONFIRM
if [[ "$CONFIRM" != "y" && "$CONFIRM" != "Y" ]]; then
    echo "Operation cancelled."
    exit 0
fi

# Update settings via API
echo "Updating auto-mark settings..."
RESPONSE=$(curl -s -X POST "https://connect.hsannu.com/api/settings/auto-mark" \
     -H "Content-Type: application/json" \
     -d "{\"hour\": $HOUR, \"minute\": $MINUTE, \"enabled\": $ENABLED}")

echo ""
echo "Response from server:"
echo "$RESPONSE"
echo ""

# Ask if user wants to run auto-marking now
read -p "Do you want to run auto-marking now? (y/n): " RUN_NOW
if [[ "$RUN_NOW" == "y" || "$RUN_NOW" == "Y" ]]; then
    echo "Triggering auto-marking now..."
    RESPONSE=$(curl -s -X POST "https://connect.hsannu.com/api/settings/auto-mark?run_now=true" \
         -H "Content-Type: application/json" \
         -d "{\"hour\": $HOUR, \"minute\": $MINUTE, \"enabled\": $ENABLED}")
    
    echo "Response from server:"
    echo "$RESPONSE"
fi

echo ""
echo "=========================================="
echo "✅ Configuration complete!"
echo "==========================================" 