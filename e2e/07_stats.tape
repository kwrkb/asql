# E2E: Column statistics overlay
# Verifies: execute query, open stats overlay, see stats table headers and columns

Output e2e/recordings/07_stats.mp4
Output e2e/recordings/07_stats.gif

Set Shell "bash"
Set Width 1400
Set Height 700
Set FontSize 16
Set TypingSpeed 50ms
Set WaitTimeout 10s

# --- Setup ---
Hide
Type "python3 docs/setup-demo-db.py /tmp/asql-e2e.db /tmp/asql-e2e-staging.db >/dev/null 2>&1"
Enter
Sleep 1s
Show

# 1. Launch asql and run a query
Type "./asql /tmp/asql-e2e.db"
Enter
Wait+Screen /INSERT/
Ctrl+l
Sleep 500ms
Type "SELECT * FROM users"
Sleep 500ms
Ctrl+j
Wait+Screen /alice@example\.com/
Sleep 1s

# 2. Open stats overlay
Escape
Wait+Screen /NORMAL/
Type "d"
Wait+Screen /Column Statistics/
Wait+Screen /NULL%/
Wait+Screen /Distinct/
Wait+Screen /email/
Wait+Screen /created_at/
Sleep 1s

# 3. Close overlay and quit
Type "q"
Sleep 500ms
Type "q"
Sleep 500ms
