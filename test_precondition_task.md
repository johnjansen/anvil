---
schedule: "*/5 * * * *"
precondition:
  day_of_week: "1-5"
  time_range: "09:00-17:00"
  env_set: "TEST_MODE"
---
echo "This task only runs on weekdays during business hours when TEST_MODE is set"