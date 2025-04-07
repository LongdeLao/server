-- Convert timestamps from 'timestamp with time zone' to 'timestamp' without timezone

-- First make a backup of the tables
CREATE TABLE messages_backup AS SELECT * FROM messages;
CREATE TABLE conversations_backup AS SELECT * FROM conversations;

-- Check the current type of created_at column
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_name = 'messages' AND column_name = 'created_at';

-- Alter messages table to change created_at to timestamp without time zone
ALTER TABLE messages 
ALTER COLUMN created_at TYPE timestamp WITHOUT TIME ZONE
USING created_at AT TIME ZONE 'UTC';

-- Alter conversations table to change created_at to timestamp without time zone
ALTER TABLE conversations 
ALTER COLUMN created_at TYPE timestamp WITHOUT TIME ZONE
USING created_at AT TIME ZONE 'UTC';

-- Update the default value for messages.created_at
ALTER TABLE messages 
ALTER COLUMN created_at SET DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')::timestamp;

-- Update the default value for conversations.created_at
ALTER TABLE conversations 
ALTER COLUMN created_at SET DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')::timestamp;

-- Verify the changes
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_name IN ('messages', 'conversations') AND column_name = 'created_at'; 