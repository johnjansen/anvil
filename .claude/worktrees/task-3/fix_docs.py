#!/usr/bin/env python3
import re

# Fix tools/skills/anvil/SKILL.md
with open('/Users/johnjansen/Documents/GitHub/johnjansen/anvil/tools/skills/anvil/SKILL.md', 'r') as f:
    content = f.read()

# Replace the provider: YAML key
content = content.replace('  provider:\n    claude:\n      requests_per_minute: 50',
                         '  provider_limits:\n    claude:\n      requests_per_minute: 50')

# Replace the provider in bullet point - try multiple patterns
content = re.sub(r'- `provider` —', '- `provider_limits` —', content)

with open('/Users/johnjansen/Documents/GitHub/johnjansen/anvil/tools/skills/anvil/SKILL.md', 'w') as f:
    f.write(content)

# Fix .claude/skills/anvil/SKILL.md
with open('/Users/johnjansen/Documents/GitHub/johnjansen/anvil/.claude/skills/anvil/SKILL.md', 'r') as f:
    content = f.read()

content = content.replace('  provider:\n    claude:\n      requests_per_minute: 50',
                         '  provider_limits:\n    claude:\n      requests_per_minute: 50')
content = re.sub(r'- `provider` —', '- `provider_limits` —', content)

with open('/Users/johnjansen/Documents/GitHub/johnjansen/anvil/.claude/skills/anvil/SKILL.md', 'w') as f:
    f.write(content)

print("Done")
