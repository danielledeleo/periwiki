PRAGMA foreign_keys = ON; -- Jesus that's stupid.

CREATE TABLE IF NOT EXISTS Article (
    id INTEGER PRIMARY KEY,
    url TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS User (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    screenname TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS Revision (
    id INTEGER NOT NULL,
    article_id INT NOT NULL,
    title TEXT NOT NULL,
    hashval TEXT NOT NULL,
    markdown TEXT NOT NULL,
    html TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    created TIMESTAMP NOT NULL,
    previous_id INT NOT NULL,
    comment TEXT,
    PRIMARY KEY (id, article_id),
    FOREIGN KEY(article_id) REFERENCES Article(id),
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
    id INT PRIMARY KEY NOT NULL,
    pref_label TEXT NOT NULL UNIQUE,
    pref_type INT NOT NULL,
    help_text TEXT,
    pref_int INT, -- type 0
    pref_text TEXT, -- type 1
    pref_selection INT -- type 2 
);

CREATE TABLE IF NOT EXISTS PreferenceSelection (
    pref_id INT,
    val INT,
    pref_selection_label TEXT,
    PRIMARY KEY (pref_id, val),
    FOREIGN KEY (pref_id) REFERENCES Preference(id)
);

CREATE TABLE IF NOT EXISTS PreferenceGroup (
    group_id INT NOT NULL,
    pref_id INT NOT NULL,
    FOREIGN KEY (pref_id) REFERENCES Preference(id)
);

CREATE TABLE IF NOT EXISTS PreferencePage (
    id INT PRIMARY KEY,
    pref_group INT NOT NULL,
    pref_namespace TEXT, 
    pref_path TEXT NOT NULL,
    template TEXT,
    title TEXT,
    FOREIGN KEY (pref_group) REFERENCES PreferenceGroup(group_id)
);

INSERT OR IGNORE INTO User(id, email, screenname) VALUES (0, "", "Anonymous");
-- INSERT OR IGNORE INTO User(id, email, screenname) VALUES (1, "", "Administrator");