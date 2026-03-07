import re
# Fix tools/skills/anvil/SKILL.md
with open('tools/skills/anvil/SKILL.md', 'r') as f:
    content = f.read()
content = re.sub(r'- `provider` —', '- `provider_limits` —', content)
with open('tools/skills/anvil/SKILL.md', 'w') as f:
    f.write(content)

# Fix .claude/skills/anvil/SKILL.md
with open('.claude/skills/anvil/SKILL.md', 'r') as f:
    content = f.read()
content = re.sub(r'- `provider` —', '- `provider_limits` —', content)
with open('.claude/skills/anvil/SKILL.md', 'w') as f:
    f.write(content)

print("Done")
