import psycopg2
"""
SQL code:
CREATE TABLE feature_flags (
  name VARCHAR(255) PRIMARY KEY,
  value BOOLEAN
);

INSERT INTO feature_flags (name, value) VALUES ('new_feature_enabled', true);

In your Python code, use a database library such as psycopg2 to connect to the database, query the feature_flags table, and use the values of the feature flags to control the behavior of your code. 
Here's an example:
"""
# Connect to the database
conn = psycopg2.connect(
    host="localhost",
    database="my_database",
    user="my_user",
    password="my_password"
)

# Query the feature_flags table
cur = conn.cursor()
cur.execute("SELECT name, value FROM feature_flags")
rows = cur.fetchall()
cur.close()

# Use the values of the feature flags to control the behavior of the code
for name, value in rows:
    if name == "new_feature_enabled" and value:
        # Run the new feature code
        print("This is the new feature!")
    elif name == "old_feature_enabled" and value:
        # Run the old feature code
        print("This is the old feature.")
