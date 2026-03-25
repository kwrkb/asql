# asql — Compare Mode Demo
Output e2e/recordings/compare-demo.gif

Set Shell "bash"
Set Width 1800
Set Height 700
Set FontSize 14
Set FontFamily "DejaVu Sans Mono"
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms
Set Padding 20

# --- Setup: create demo databases ---
Hide
Set TypingSpeed 1ms
Type "python3 docs/setup-demo-db.py /tmp/asql-demo.db /tmp/asql-demo-staging.db >/dev/null 2>&1"
Enter
Type "python3 e2e/setup-profiles.py /tmp/asql-demo-config /tmp/asql-demo.db /tmp/asql-demo-staging.db >/dev/null 2>&1"
Enter
Sleep 2s
Show
Set TypingSpeed 30ms

# --- Demo starts ---

# 1. Launch asql with production profile
Type "XDG_CONFIG_HOME=/tmp/asql-demo-config ./asql @prod"
Enter
Sleep 2s

# 2. Clear initial content and write a query for users table
Ctrl+l
Sleep 500ms
Type "SELECT * FROM users"
Sleep 500ms

# 3. Execute query
Ctrl+j
Sleep 2s

# 4. Enter compare mode (pin production result to left pane)
Escape
Sleep 1s
Type "c"
Sleep 2s

# 5. Open profile list, staging is auto-selected, switch+exec with 'x'
Type "P"
Sleep 1s
Type "x"
Sleep 2.5s

# 7. Pause to show the diff highlights (row count + cell differences)
Sleep 4s
