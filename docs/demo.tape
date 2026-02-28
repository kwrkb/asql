# asql — TUI SQL Client Demo
Output docs/demo.gif

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set FontFamily "DejaVu Sans Mono"
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms
Set Padding 20

# --- Setup: create demo database with sample tables ---
Hide
Set TypingSpeed 1ms
Type "python3 docs/setup-demo-db.py /tmp/asql-demo.db"
Enter
Sleep 500ms
Type "clear"
Enter
Sleep 200ms
Show
Set TypingSpeed 30ms

# --- Demo starts ---

# 1. Launch asql (INSERT mode, initial query visible)
Type "./asql /tmp/asql-demo.db"
Enter
Sleep 2s

# 2. Execute initial query (SELECT sqlite_version())
Ctrl+j
Sleep 2s

# 3. Switch to NORMAL mode (status bar hints change)
Escape
Sleep 1.5s

# 4. Open table sidebar
Type "t"
Sleep 1.5s

# 5. Navigate sidebar to 'users' (posts → tags → users)
Type "j"
Sleep 500ms
Type "j"
Sleep 1s

# 6. Select table → inserts SELECT query, enters INSERT mode
Enter
Sleep 1.5s

# 7. Execute the query
Ctrl+j
Sleep 3s
