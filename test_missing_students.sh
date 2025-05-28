#!/bin/bash

# Script to test the Missing Students API

# Base URL
BASE_URL="http://localhost:2000/api/missing-students"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== Testing Missing Students API ===${NC}\n"

# 1. Get Year Group Coordinators
echo -e "${YELLOW}1. Getting Year Group Coordinators...${NC}"
curl -s -X GET "$BASE_URL/coordinators?user_id=33" | jq '.'
echo -e "\n"

# 2. Report a missing student (Selina reporting a student from IB1)
echo -e "${YELLOW}2. Reporting a missing student from IB1 (by Selina)...${NC}"
curl -s -X POST "$BASE_URL/report?reporter_id=33" \
  -H "Content-Type: application/json" \
  -d '{
    "student_id": 101,
    "notes": "Student did not attend morning class"
  }' | jq '.'
echo -e "\n"

# 3. Report another missing student (Vermouth reporting a student from PIB)
echo -e "${YELLOW}3. Reporting a missing student from PIB (by Vermouth)...${NC}"
curl -s -X POST "$BASE_URL/report?reporter_id=77" \
  -H "Content-Type: application/json" \
  -d '{
    "student_id": 102,
    "notes": "Student missing from afternoon class"
  }' | jq '.'
echo -e "\n"

# 4. Get all missing students (as attendance admin)
echo -e "${YELLOW}4. Getting all missing students (as attendance admin)...${NC}"
curl -s -X GET "$BASE_URL?user_id=33&status=reported" | jq '.'
echo -e "\n"

# 5. Filter missing students by year group
echo -e "${YELLOW}5. Filtering missing students by PIB year group...${NC}"
curl -s -X GET "$BASE_URL?user_id=77&year_group=PIB" | jq '.'
echo -e "\n"

# 6. Resolve a missing student
echo -e "${YELLOW}6. Resolving a missing student report...${NC}"
# Note: You need to replace 1 with an actual report ID from previous responses
curl -s -X POST "$BASE_URL/resolve/1?resolver_id=33" \
  -H "Content-Type: application/json" \
  -d '{
    "notes": "Student found in the library"
  }' | jq '.'
echo -e "\n"

# 7. Get resolved reports
echo -e "${YELLOW}7. Getting resolved reports...${NC}"
curl -s -X GET "$BASE_URL?user_id=33&status=resolved" | jq '.'
echo -e "\n"

echo -e "${GREEN}Tests completed!${NC}" 