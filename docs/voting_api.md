# Voting System API Documentation

This document outlines the API endpoints available for the HSANNU voting system.

## Table of Contents
1. [Events Management](#events-management)
2. [User Votes](#user-votes)
3. [Statistics & Results](#statistics-results)

## Base URL

All API endpoints are prefixed with `/api`.

## Authentication

All endpoints require a `User-ID` header with a valid user ID.

## Events Management

### Get All Voting Events

Retrieves all voting events with their sub-votes and options.

**URL**: `/voting/events`

**Method**: `GET`

**Query Parameters**:
- `status` (optional): Filter events by status (e.g., "active", "completed")
- `user_id` (optional): User ID to check if they have voted in each sub-vote

**Response**:
```json
[
  {
    "id": 1,
    "title": "Winter Fantasy",
    "description": "Help us plan the Winter Fantasy event",
    "deadline": "2024-11-15T23:59:59Z",
    "status": "active",
    "organizer_id": 42,
    "organizer_name": "Hyunha",
    "organizer_role": "Entertainment Department",
    "vote_count": 78,
    "total_votes": 160,
    "created_at": "2024-05-01T12:00:00Z",
    "sub_votes": [
      {
        "id": 1,
        "event_id": 1,
        "title": "Event Location",
        "description": "Choose the venue for our Winter Fantasy event",
        "options": [
          {
            "id": 1,
            "sub_vote_id": 1,
            "text": "Music Hall",
            "has_custom_input": false,
            "vote_count": 25
          },
          {
            "id": 2,
            "sub_vote_id": 1,
            "text": "4th Floor Hall",
            "has_custom_input": false,
            "vote_count": 40
          }
        ]
      }
    ]
  }
]
```

### Get Voting Event by ID

Retrieves a single voting event with its sub-votes and options.

**URL**: `/voting/events/:id`

**Method**: `GET`

**URL Parameters**:
- `id`: ID of the voting event

**Query Parameters**:
- `user_id` (optional): User ID to check if they have voted in each sub-vote

**Response**: Same as Get All Voting Events but for a single event.

### Create Voting Event

Creates a new voting event with its sub-votes and options.

**URL**: `/voting/events`

**Method**: `POST`

**Headers**:
- `User-ID`: ID of the organizer creating the event

**Request Body**:
```json
{
  "title": "School Day Trip",
  "description": "Vote on where we should organize our upcoming school day trip",
  "deadline": "2025-07-07T23:59:59Z",
  "status": "active",
  "sub_votes": [
    {
      "title": "Trip Destination",
      "description": "Where should we go for our day trip?",
      "options": [
        {
          "text": "South Lake",
          "has_custom_input": false
        },
        {
          "text": "Jingyue Park",
          "has_custom_input": false
        },
        {
          "text": "Suggest a place",
          "has_custom_input": true
        }
      ]
    },
    {
      "title": "Food Options",
      "description": "What food should be provided during the trip?",
      "options": [
        {
          "text": "Packed Lunch Boxes",
          "has_custom_input": false
        },
        {
          "text": "BBQ at Location",
          "has_custom_input": false
        }
      ]
    }
  ]
}
```

**Response**:
```json
{
  "message": "Voting event created successfully",
  "event_id": 3
}
```

### Update Voting Event

Updates an existing voting event.

**URL**: `/voting/events/:id`

**Method**: `PUT`

**URL Parameters**:
- `id`: ID of the voting event to update

**Headers**:
- `User-ID`: ID of the organizer updating the event

**Request Body**: Same as Create Voting Event

**Response**:
```json
{
  "message": "Voting event updated successfully"
}
```

### Delete Voting Event

Deletes a voting event along with its sub-votes, options, and related votes.

**URL**: `/voting/events/:id`

**Method**: `DELETE`

**URL Parameters**:
- `id`: ID of the voting event to delete

**Response**:
```json
{
  "message": "Voting event deleted successfully"
}
```

## User Votes

### Submit Vote

Submits a user's vote for a specific sub-vote and option.

**URL**: `/voting/vote`

**Method**: `POST`

**Headers**:
- `User-ID`: ID of the user submitting the vote

**Request Body**:
```json
{
  "sub_vote_id": 1,
  "option_id": 2,
  "custom_input": "My custom suggestion" // Optional, only used if option has has_custom_input=true
}
```

**Response**:
```json
{
  "message": "Vote submitted successfully",
  "updated": false // true if the user had already voted and this is an update
}
```

### Check User Vote

Checks if a user has already voted in a specific sub-vote.

**URL**: `/voting/check-vote`

**Method**: `GET`

**Query Parameters**:
- `user_id`: User ID to check
- `sub_vote_id`: Sub vote ID to check

**Response**:
```json
{
  "has_voted": true,
  "vote_details": {
    "vote_id": 123,
    "option_id": 2,
    "option_text": "4th Floor Hall",
    "custom_input": "My custom suggestion"
  }
}
```

### Get User Votes

Retrieves all votes submitted by a specific user.

**URL**: `/voting/user-votes/:user_id`

**Method**: `GET`

**URL Parameters**:
- `user_id`: ID of the user

**Response**:
```json
[
  {
    "id": 123,
    "user_id": 42,
    "sub_vote_id": 1,
    "option_id": 2,
    "custom_input": "My custom suggestion",
    "created_at": "2024-05-05T15:30:00Z",
    "sub_vote_title": "Event Location",
    "option_text": "4th Floor Hall"
  }
]
```

### Delete User Vote

Deletes a user's vote.

**URL**: `/voting/user-votes/:id`

**Method**: `DELETE`

**URL Parameters**:
- `id`: ID of the vote to delete

**Response**:
```json
{
  "message": "Vote deleted successfully"
}
```

## Statistics & Results

### Get Vote Results

Gets detailed results for a voting event, including vote counts and percentages.

**URL**: `/voting/results/:event_id`

**Method**: `GET`

**URL Parameters**:
- `event_id`: ID of the voting event

**Response**:
```json
{
  "event": {
    "id": 1,
    "title": "Winter Fantasy",
    "description": "Help us plan the Winter Fantasy event",
    "deadline": "2024-11-15T23:59:59Z",
    "status": "active",
    "organizer_id": 42,
    "organizer_name": "Hyunha",
    "organizer_role": "Entertainment Department",
    "created_at": "2024-05-01T12:00:00Z",
    "vote_count": 78,
    "total_votes": 160
  },
  "results": [
    {
      "sub_vote_id": 1,
      "title": "Event Location",
      "description": "Choose the venue for our Winter Fantasy event",
      "total_votes": 65,
      "options": [
        {
          "option_id": 1,
          "text": "Music Hall",
          "has_custom_input": false,
          "vote_count": 25,
          "percentage": 38.46
        },
        {
          "option_id": 2,
          "text": "4th Floor Hall",
          "has_custom_input": false,
          "vote_count": 40,
          "percentage": 61.54
        }
      ]
    },
    {
      "sub_vote_id": 2,
      "title": "Event Games",
      "description": "Select your favorite game for the Winter Fantasy",
      "total_votes": 55,
      "options": [
        {
          "option_id": 3,
          "text": "Musical Chairs",
          "has_custom_input": false,
          "vote_count": 20,
          "percentage": 36.36
        },
        {
          "option_id": 4,
          "text": "Blindfolded Hitting",
          "has_custom_input": false,
          "vote_count": 10,
          "percentage": 18.18
        },
        {
          "option_id": 5,
          "text": "Suggest your own idea",
          "has_custom_input": true,
          "vote_count": 25,
          "percentage": 45.45,
          "custom_inputs": [
            "Karaoke",
            "Treasure hunt",
            "Dance competition"
          ]
        }
      ]
    }
  ]
}
```

### Get Voting Statistics

Gets general statistics for a voting event.

**URL**: `/voting/statistics/:event_id`

**Method**: `GET`

**URL Parameters**:
- `event_id`: ID of the voting event

**Response**: Similar to Get Vote Results but with simplified statistics. 