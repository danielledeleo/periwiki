PRAGMA foreign_keys = ON; -- Jesus that's stupid.

CREATE TABLE IF NOT EXISTS Article (
    id INTEGER PRIMARY KEY NOT NULL,
    url TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS User (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    screenname TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS Revision (
    id INTEGER PRIMARY KEY NOT NULL,
    title TEXT NOT NULL,
    hashval TEXT NOT NULL,
    markdown TEXT NOT NULL,
    html TEXT NOT NULL,
    article_id INT NOT NULL,
    user_id INTEGER NOT NULL,
    created TIMESTAMP NOT NULL,
    previous_id INT NOT NULL,
    comment TEXT,
    FOREIGN KEY(article_id) REFERENCES Article(id),
    -- FOREIGN KEY(previous_id) REFERENCES Revision(id),
    FOREIGN KEY(user_id) REFERENCES User(id)
);

CREATE TABLE IF NOT EXISTS Password (
    user_id INTEGER PRIMARY KEY NOT NULL,
    passwordhash TEXT NOT NULL,
    FOREIGN KEY(user_id) REFERENCES User(id)
);

CREATE TABLE IF NOT EXISTS AnonymousEdit (
    ip TEXT NOT NULL,
    revision_id INT NOT NULL,
    FOREIGN KEY(revision_id) REFERENCES Revision(id)
);

CREATE TABLE IF NOT EXISTS Preference (
    pref TEXT PRIMARY KEY NOT NULL,
    val TEXT
);

INSERT OR IGNORE INTO User(id, email, screenname) VALUES (0, "", "Anonymous");
-- INSERT OR IGNORE INTO User(id, email, screenname) VALUES (1, "", "Administrator");