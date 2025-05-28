# Missing Students Reporting System

This document provides information about the Missing Students Reporting System implemented in the HSANNU application.

## Overview

The Missing Students Reporting System allows staff members to report and track students who are missing from classes. The system has two main components:

1. **Year Group Coordinators**: Staff members assigned to specific year groups (PIB, IB1) who can report and track missing students from their assigned year groups.
2. **Missing Students Reports**: Records of students reported as missing, which can be resolved when the students are found.

## Database Tables

The system uses two main database tables:

### 1. Year Group Coordinators Table

This table stores the assignments of staff members to specific year groups:

```sql
CREATE TABLE year_group_coordinators (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    year_group VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_user_year_group UNIQUE(user_id, year_group)
);
```

Current assignments:
- Selina (user_id: 33) - IB1
- Linda (user_id: 86) - IB1
- Vermouth (user_id: 77) - PIB
- Cathy (user_id: 32) - PIB

### 2. Missing Students Table

This table stores reports of missing students:

```sql
CREATE TABLE missing_students (
    id SERIAL PRIMARY KEY,
    student_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reported_by INTEGER NOT NULL REFERENCES users(id),
    year_group VARCHAR(255) NOT NULL,
    report_date DATE NOT NULL DEFAULT CURRENT_DATE,
    report_time TIME NOT NULL DEFAULT CURRENT_TIME,
    status VARCHAR(50) NOT NULL DEFAULT 'reported',
    notes TEXT,
    resolved_by INTEGER REFERENCES users(id),
    resolved_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## API Endpoints

The system provides the following API endpoints:

### 1. Report a Missing Student

**Endpoint:** `POST /api/missing-students/report`

**Query Parameters:**
- `reporter_id` (required): The ID of the staff member reporting the student

**Request Body:**
```json
{
  "student_id": 123,
  "notes": "Student did not attend morning class"
}
```

### 2. Get Missing Students

**Endpoint:** `GET /api/missing-students`

**Query Parameters:**
- `user_id` (required): The ID of the staff member making the request
- `status` (optional): Filter by status (e.g., "reported", "resolved")
- `year_group` (optional): Filter by year group (e.g., "PIB", "IB1")

### 3. Resolve a Missing Student Report

**Endpoint:** `POST /api/missing-students/resolve/:id`

**Path Parameters:**
- `id`: The ID of the missing student report to resolve

**Query Parameters:**
- `resolver_id` (required): The ID of the staff member resolving the report

**Request Body:**
```json
{
  "notes": "Student found in the library"
}
```

### 4. Get Year Group Coordinators

**Endpoint:** `GET /api/missing-students/coordinators`

**Query Parameters:**
- `user_id` (required): The ID of the staff member making the request

## Permissions

- **Year Group Coordinators** can report and view missing students from their assigned year groups
- Staff members with the **attendance** role can report and view missing students from all year groups
- Only **staff members** (either coordinators or with attendance role) can resolve missing student reports

## Testing

A test script is provided to verify the API functionality:

```bash
./test_missing_students.sh
```

## Implementation Notes

- The system automatically detects the year group of a student from the attendance database
- Reports can be filtered by status ("reported" or "resolved") and by year group
- Notes can be added when reporting a student as missing and when resolving a report
- Staff members can only view reports for their assigned year groups unless they have the attendance role 