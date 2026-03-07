---
schedule: "*/30 * * * *"
priority_aging:
  enabled: true
  max_wait: 1h
  max_boost: 2
---
This is a sample task that demonstrates priority aging.
It will be boosted in priority if it waits in the queue for more than 1 hour.