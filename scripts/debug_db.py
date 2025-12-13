import sqlite3
import os

db_path = '/projects/Charon/backend/data/charon.db'

if not os.path.exists(db_path):
    print(f"Database not found at {db_path}")
    exit(1)

try:
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()

    cursor.execute("SELECT id, domain_names, forward_host, forward_port FROM proxy_hosts")
    rows = cursor.fetchall()

    print("Proxy Hosts:")
    for row in rows:
        print(f"ID: {row[0]}, Domains: {row[1]}, ForwardHost: {row[2]}, Port: {row[3]}")

    conn.close()
except Exception as e:
    print(f"Error: {e}")
