#!/usr/bin/env python3
"""Create a demo SQLite database for VHS recording."""
import os
import sqlite3
import sys

db_path = sys.argv[1] if len(sys.argv) > 1 else "/tmp/asql-demo.db"
if os.path.exists(db_path):
    os.remove(db_path)

conn = sqlite3.connect(db_path)
conn.executescript("""
CREATE TABLE users (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT UNIQUE,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE posts (
    id        INTEGER PRIMARY KEY,
    user_id   INTEGER REFERENCES users(id),
    title     TEXT NOT NULL,
    body      TEXT,
    published BOOLEAN DEFAULT 0
);
CREATE TABLE tags (
    id    INTEGER PRIMARY KEY,
    name  TEXT NOT NULL UNIQUE,
    color TEXT
);

INSERT INTO users (name, email) VALUES
    ('Alice',   'alice@example.com'),
    ('Bob',     'bob@example.com'),
    ('Charlie', 'charlie@example.com');

INSERT INTO posts (user_id, title, body, published) VALUES
    (1, 'Getting Started with SQL', 'A beginner guide',             1),
    (1, 'Advanced Queries',         'Deep dive into joins',         1),
    (2, 'Database Design',          'Best practices for schemas',   0);

INSERT INTO tags (name, color) VALUES
    ('sql',      '#3B82F6'),
    ('tutorial', '#10B981'),
    ('database', '#F59E0B');
""")
conn.close()
