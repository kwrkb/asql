# E2E: Read-only session
# Verifies: the ro marker in the status bar, and that a writing statement is
# refused with a message naming what was rejected

Output e2e/recordings/08_readonly.mp4
Output e2e/recordings/08_readonly.gif

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set TypingSpeed 50ms
Set WaitTimeout 10s

# --- Setup ---
Hide
Type "python3 docs/setup-demo-db.py /tmp/asql-e2e.db /tmp/asql-e2e-staging.db >/dev/null 2>&1"
Enter
Sleep 1s
Show

# 1. Launch read-only
Type "./asql --readonly /tmp/asql-e2e.db"
Enter
Wait+Screen /INSERT/

# 2. The status bar marks the connection read-only
Wait+Screen /SQLITE ro/
Sleep 1s

# 3. A read is still a read
Ctrl+l
Sleep 500ms
Type "SELECT id, name FROM users LIMIT 5"
Sleep 500ms
Ctrl+j
Wait+Screen /Alice/
Sleep 1s

# 4. A write is refused before it reaches the database
Ctrl+l
Sleep 500ms
Type "DELETE FROM users"
Sleep 500ms
Ctrl+j
Wait+Screen /readonly: DELETE is not allowed/
Sleep 1s

# 5. So is a statement that hides the write inside a CTE
Ctrl+l
Sleep 500ms
Type "WITH gone AS (DELETE FROM users RETURNING *) SELECT * FROM gone"
Sleep 500ms
Ctrl+j
Wait+Screen /readonly: DELETE is not allowed/
Sleep 1s

# 6. Quit
Escape
Sleep 500ms
Type "q"
Sleep 500ms
