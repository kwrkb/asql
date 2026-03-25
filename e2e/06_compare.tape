# E2E: Compare mode across profiles
# Verifies: pin result, switch profile, re-execute, see row-count and cell differences

Output e2e/recordings/06_compare.mp4
Output e2e/recordings/06_compare.gif

Set Shell "bash"
Set Width 1800
Set Height 700
Set FontSize 14
Set TypingSpeed 50ms
Set WaitTimeout 10s

# --- Setup ---
Hide
Type "python3 docs/setup-demo-db.py /tmp/asql-e2e.db /tmp/asql-e2e-staging.db >/dev/null 2>&1"
Enter
Type "python3 e2e/setup-profiles.py /tmp/asql-e2e-config /tmp/asql-e2e.db /tmp/asql-e2e-staging.db >/dev/null 2>&1"
Enter
Sleep 1s
Show

# 1. Launch prod profile
Type "XDG_CONFIG_HOME=/tmp/asql-e2e-config ./asql @prod"
Enter
Wait+Screen /INSERT/

# 2. Run users query
Ctrl+l
Sleep 500ms
Type "SELECT * FROM users"
Sleep 500ms
Ctrl+j
Wait+Screen /bob@example\.com/
Sleep 1s

# 3. Pin current result for compare
Escape
Wait+Screen /NORMAL/
Type "c"
Wait+Screen /Compare rows left:3 right:3 diff:\+0/
Sleep 1s

# 4. Switch to staging and re-execute
Type "P"
Wait+Screen /Connection Profiles/
Type "j"
Sleep 500ms
Type "x"
Wait+Screen /staging rows:4/
Wait+Screen /Compare rows left:3 right:4 diff:\+1/
Sleep 1s

# 5. Quit
Type "q"
Sleep 500ms
